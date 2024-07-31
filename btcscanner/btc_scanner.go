package btcscanner

import (
	"fmt"
	"sync"
	"time"

	notifier "github.com/lightningnetwork/lnd/chainntnfs"
	"go.uber.org/atomic"
	"go.uber.org/zap"

	"github.com/babylonlabs-io/staking-indexer/types"
)

const ConfirmedBlockBatchSize = 100

var _ BtcScanner = (*BtcPoller)(nil)

type BtcScanner interface {
	Start(startHeight, activationHeight uint64) error

	// ChainUpdateInfoChan receives the chain update info
	// after bootstrapping or when new block is received
	ChainUpdateInfoChan() <-chan *ChainUpdateInfo

	LastConfirmedHeight() uint64

	Stop() error
}

type ChainUpdateInfo struct {
	ConfirmedBlocks   []*types.IndexedBlock
	UnconfirmedBlocks []*types.IndexedBlock
}

type BtcPoller struct {
	logger *zap.Logger

	// connect to BTC node
	btcClient   Client
	btcNotifier notifier.ChainNotifier

	confirmationDepth uint16

	// the current tip BTC block
	confirmedTipBlock *types.IndexedBlock

	// cache of a sequence of unconfirmed blocks
	unconfirmedBlockCache *BTCCache

	// receives chain update info
	chainUpdateInfoChan chan *ChainUpdateInfo

	wg        sync.WaitGroup
	isStarted *atomic.Bool
	quit      chan struct{}
}

func NewBTCScanner(
	confirmationDepth uint16,
	logger *zap.Logger,
	btcClient Client,
	btcNotifier notifier.ChainNotifier,
) (*BtcPoller, error) {
	unconfirmedBlockCache, err := NewBTCCache(defaultMaxEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to create BTC cache for tail blocks: %w", err)
	}

	return &BtcPoller{
		logger:                logger.With(zap.String("module", "btcscanner")),
		btcClient:             btcClient,
		btcNotifier:           btcNotifier,
		confirmationDepth:     confirmationDepth,
		chainUpdateInfoChan:   make(chan *ChainUpdateInfo),
		unconfirmedBlockCache: unconfirmedBlockCache,
		isStarted:             atomic.NewBool(false),
		quit:                  make(chan struct{}),
	}, nil
}

// Start starts the scanning process from the last confirmed height + 1
func (bs *BtcPoller) Start(startHeight, activationHeight uint64) error {
	if bs.isStarted.Swap(true) {
		return fmt.Errorf("the BTC scanner is already started")
	}

	if err := bs.waitUntilActivation(activationHeight); err != nil {
		return err
	}

	bs.logger.Info("starting the BTC scanner", zap.Uint64("start_height", startHeight))

	if err := bs.Bootstrap(startHeight); err != nil {
		return fmt.Errorf("failed to bootstrap with height %d: %w", startHeight, err)
	}

	// start handling new blocks
	bs.wg.Add(1)
	go bs.blockEventLoop(startHeight)

	bs.logger.Info("the BTC scanner is started")

	return nil
}

func (bs *BtcPoller) waitUntilActivation(activationHeight uint64) error {
	for {
		tipHeight, err := bs.btcClient.GetTipHeight()
		if err != nil {
			return fmt.Errorf("failed to get the current BTC tip height")
		}

		if tipHeight >= activationHeight {
			break
		}

		bs.logger.Info("waiting to reach the earliest activation height",
			zap.Uint64("tip_height", tipHeight),
			zap.Uint64("activation_height", activationHeight))
		time.Sleep(10 * time.Second)
	}

	return nil
}

// Bootstrap syncs with BTC by getting the confirmed blocks and the caching the unconfirmed blocks
func (bs *BtcPoller) Bootstrap(startHeight uint64) error {
	bs.logger.Info("the bootstrapping starts", zap.Uint64("start_height", startHeight))

	// clear all the blocks in the cache to avoid forks
	bs.unconfirmedBlockCache.RemoveAll()

	tipHeight, err := bs.btcClient.GetTipHeight()
	if err != nil {
		return fmt.Errorf("cannot get the best BTC block")
	}

	if startHeight > tipHeight {
		return fmt.Errorf("the start height %d is higher than the current tip height %d", startHeight, tipHeight)
	}

	var confirmedBlocks []*types.IndexedBlock
	for i := startHeight; i <= tipHeight; i++ {
		ib, err := bs.btcClient.GetBlockByHeight(i)
		if err != nil {
			return fmt.Errorf("cannot get the block at height %d: %w", i, err)
		}

		// the unconfirmed blocks should follow the canonical chain
		tipCache := bs.unconfirmedBlockCache.Tip()
		if tipCache != nil {
			tipHash := tipCache.BlockHash()
			if !tipHash.IsEqual(&ib.Header.PrevBlock) {
				return fmt.Errorf("the block is not connected to the cache tip")
			}
		}

		if err := bs.unconfirmedBlockCache.Add(ib); err != nil {
			return fmt.Errorf("failed to add the block %d to cache: %w", ib.Height, err)
		}

		tempConfirmedBlocks := bs.unconfirmedBlockCache.TrimConfirmedBlocks(int(bs.confirmationDepth) - 1)
		confirmedBlocks = append(confirmedBlocks, tempConfirmedBlocks...)

		// commit a batch to free up memory
		if len(confirmedBlocks) >= ConfirmedBlockBatchSize {
			// deep copy so that the copy will not be affected by memory release
			blocksCopy := make([]*types.IndexedBlock, len(confirmedBlocks))
			copy(blocksCopy, confirmedBlocks)
			bs.commitChainUpdate(blocksCopy)

			confirmedBlocks = nil
		}
	}

	if len(confirmedBlocks) != 0 || len(bs.getUnconfirmedBlocks()) != 0 {
		bs.commitChainUpdate(confirmedBlocks)
	}

	bs.logger.Info("bootstrapping is finished",
		zap.Uint64("tip_unconfirmed_height", tipHeight))

	return nil
}

func (bs *BtcPoller) getUnconfirmedBlocks() []*types.IndexedBlock {
	tipBlock := bs.unconfirmedBlockCache.Tip()
	if tipBlock == nil {
		return nil
	}

	lastBlocks := bs.unconfirmedBlockCache.GetLastBlocks(int(bs.confirmationDepth) - 1)

	return lastBlocks
}

func (bs *BtcPoller) ChainUpdateInfoChan() <-chan *ChainUpdateInfo {
	return bs.chainUpdateInfoChan
}

func (bs *BtcPoller) LastConfirmedHeight() uint64 {
	if bs.confirmedTipBlock == nil {
		return 0
	}
	return uint64(bs.confirmedTipBlock.Height)
}

func (bs *BtcPoller) Stop() error {
	if !bs.isStarted.Swap(false) {
		return nil
	}

	close(bs.quit)
	bs.wg.Wait()

	bs.logger.Info("the BTC scanner is successfully stopped")

	return nil
}
