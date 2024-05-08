package indexer

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/babylonchain/babylon/btcstaking"
	queuecli "github.com/babylonchain/staking-queue-client/client"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/kvdb"
	"go.uber.org/zap"

	"github.com/babylonchain/staking-indexer/btcscanner"
	"github.com/babylonchain/staking-indexer/config"
	"github.com/babylonchain/staking-indexer/consumer"
	"github.com/babylonchain/staking-indexer/indexerstore"
	"github.com/babylonchain/staking-indexer/types"
)

type StakingIndexer struct {
	startOnce sync.Once
	stopOnce  sync.Once

	consumer       consumer.EventConsumer
	paramsVersions *types.ParamsVersions

	cfg    *config.Config
	logger *zap.Logger

	is *indexerstore.IndexerStore

	btcScanner btcscanner.BtcScanner

	wg   sync.WaitGroup
	quit chan struct{}
}

func NewStakingIndexer(
	cfg *config.Config,
	logger *zap.Logger,
	consumer consumer.EventConsumer,
	db kvdb.Backend,
	paramsVersions *types.ParamsVersions,
	btcScanner btcscanner.BtcScanner,
) (*StakingIndexer, error) {
	is, err := indexerstore.NewIndexerStore(db)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate staking indexer store: %w", err)
	}

	return &StakingIndexer{
		cfg:            cfg,
		logger:         logger.With(zap.String("module", "staking indexer")),
		consumer:       consumer,
		is:             is,
		paramsVersions: paramsVersions,
		btcScanner:     btcScanner,
		quit:           make(chan struct{}),
	}, nil
}

// Start starts the staking indexer core
func (si *StakingIndexer) Start(startHeight uint64) error {
	var startErr error
	si.startOnce.Do(func() {
		si.logger.Info("Starting Staking Indexer App")

		si.wg.Add(1)
		go si.confirmedBlocksLoop()

		if err := si.ValidateStartHeight(startHeight); err != nil {
			startErr = fmt.Errorf("invalid start height %d: %w", startHeight, err)
			return
		}

		if err := si.btcScanner.Start(startHeight); err != nil {
			startErr = err
			return
		}

		// record metrics
		startBtcHeight.Set(float64(startHeight))

		si.logger.Info("Staking Indexer App is successfully started!")
	})

	return startErr
}

// ValidateStartHeight validates the given startHeight and returns an error
// if the given startHeight is not in the range of
// [base height, last processed height + 1]
// The point of this validation is to ensure the indexer
// (1) does not handle irrelevant blocks (impossible to have staking tx)
// (2) does not miss relevant blocks (possible to have staking tx)
func (si *StakingIndexer) ValidateStartHeight(startHeight uint64) error {
	baseHeight := si.cfg.BTCScannerConfig.BaseHeight
	if startHeight < baseHeight {
		return fmt.Errorf("the start height should not be lower than the base height %d", baseHeight)
	}

	lastProcessedHeight, err := si.is.GetLastProcessedHeight()
	if err != nil && startHeight != baseHeight {
		return fmt.Errorf("the database is empty, the start height should be equal to the base height %d", baseHeight)
	}

	if lastProcessedHeight != 0 && startHeight > lastProcessedHeight+1 {
		return fmt.Errorf("the start height should not be higher than %d (the last processed height + 1)", lastProcessedHeight+1)
	}

	return nil
}

// GetStartHeight returns a start height that can pass ValidateStartHeight()
// if the database is empty, then the base height in the config will be returned
// otherwise, it will return the last processed height + 1
func (si *StakingIndexer) GetStartHeight() uint64 {
	lastProcessedHeight, err := si.is.GetLastProcessedHeight()
	if err != nil {
		return si.cfg.BTCScannerConfig.BaseHeight
	}

	return lastProcessedHeight + 1
}

func (si *StakingIndexer) confirmedBlocksLoop() {
	defer si.wg.Done()

	for {
		select {
		case block := <-si.btcScanner.ConfirmedBlocksChan():
			b := block
			si.logger.Info("received confirmed block",
				zap.Int32("height", block.Height))
			if err := si.HandleConfirmedBlock(b); err != nil {
				// this indicates systematic failure
				si.logger.Fatal("failed to handle block", zap.Error(err))
			}
		case <-si.quit:
			si.logger.Info("closing the confirmed blocks loop")
			return
		}
	}
}

// HandleConfirmedBlock iterates through the tx set of a confirmed block and
// parse the staking, unbonding, and withdrawal txs if there are any.
func (si *StakingIndexer) HandleConfirmedBlock(b *types.IndexedBlock) error {
	params, err := si.paramsVersions.GetParamsForBTCHeight(b.Height)
	if err != nil {
		return err
	}
	for _, tx := range b.Txs {
		msgTx := tx.MsgTx()

		// 0. check whether the tx has already processed
		processed, err := si.IsTxProcessed(tx.Hash())
		if err != nil {
			// indicates the db is corrupted
			return err
		}
		if processed {
			continue
		}

		// 1. try to parse staking tx
		stakingData, err := si.tryParseStakingTx(msgTx, params)
		if err == nil {
			if err := si.ProcessStakingTx(
				msgTx, stakingData, uint64(b.Height), b.Header.Timestamp, params,
			); err != nil {
				if errors.Is(err, ErrInvalidStakingTx) {
					invalidStakingTxsCounter.Inc()
					si.logger.Error("found an invalid staking tx",
						zap.String("tx_hash", msgTx.TxHash().String()),
						zap.Error(err),
					)
					// We will continue to the next tx as the staking tx is invalid
					// and we don't want to stop the indexer
					continue
				} else {
					// record metrics
					failedProcessingStakingTxsCounter.Inc()

					return err
				}
			}

			// should not use *continue* here as a special case is
			// the tx could be a staking tx as well as a withdrawal
			// tx that spends the previous staking tx
		}

		// 2. not a staking tx, check whether it spends a stored staking tx
		stakingTx, spentInputIdx := si.getSpentStakingTx(msgTx)
		if spentInputIdx >= 0 {
			stakingTxHash := stakingTx.Tx.TxHash()
			paramsFromStakingTxHeight, err := si.paramsVersions.GetParamsForBTCHeight(
				int32(stakingTx.InclusionHeight),
			)
			if err != nil {
				return fmt.Errorf("failed to get the params for the staking tx height: %w", err)
			}
			// 3. is a spending tx, check whether it is a valid unbonding tx
			isUnbonding, err := si.IsValidUnbondingTx(msgTx, stakingTx, paramsFromStakingTxHeight)
			if err != nil {
				if errors.Is(err, ErrInvalidUnbondingTx) {
					invalidUnbondingTxsCounter.Inc()
					si.logger.Error("found an invalid unbonding tx",
						zap.String("tx_hash", msgTx.TxHash().String()),
						zap.Error(err),
					)

					continue
				}
				// record metrics
				failedVerifyingUnbondingTxsCounter.Inc()
				return err
			}
			if !isUnbonding {
				// 4. not an unbongidng tx, so this is a withdraw tx from the staking
				// TODO we should check if it indeed unlocks the timelock path and raise alarm if not
				if err := si.processWithdrawTx(msgTx, &stakingTxHash, nil, uint64(b.Height)); err != nil {
					// record metrics
					failedProcessingWithdrawTxsFromStakingCounter.Inc()

					return err
				}
				continue
			}

			// 5. this is an unbonding tx
			if err := si.ProcessUnbondingTx(
				msgTx, &stakingTxHash, uint64(b.Height), b.Header.Timestamp,
				paramsFromStakingTxHeight,
			); err != nil {
				if !errors.Is(err, indexerstore.ErrDuplicateTransaction) {
					// record metrics
					failedProcessingUnbondingTxsCounter.Inc()

					return err
				}
				// we don't consider duplicate error critical as it can happen
				// when the indexer restarts
				si.logger.Warn("found a duplicate tx",
					zap.String("tx_hash", msgTx.TxHash().String()))
			}
			continue
		}

		// 6. it does not spend staking tx, check whether it spends stored
		// unbonding tx
		unbondingTx, spentInputIdx := si.getSpentUnbondingTx(msgTx)
		if spentInputIdx >= 0 {
			// TODO we should check if it indeed unlocks the time lock path and raise alarm if not
			// 7. this is a withdraw tx from the unbonding
			unbondingTxHash := unbondingTx.Tx.TxHash()
			if err := si.processWithdrawTx(msgTx, unbondingTx.StakingTxHash, &unbondingTxHash, uint64(b.Height)); err != nil {
				// record metrics
				failedProcessingWithdrawTxsFromUnbondingCounter.Inc()

				return err
			}
		}
	}

	if err := si.is.SaveLastProcessedHeight(uint64(b.Height)); err != nil {
		return fmt.Errorf("failed to save the last processed height: %w", err)
	}

	// record metrics
	lastProcessedBtcHeight.Set(float64(b.Height))

	return nil
}

func (si *StakingIndexer) IsTxProcessed(txHash *chainhash.Hash) (bool, error) {
	return si.is.TxExists(txHash)
}

// getSpentStakingTx checks if the given tx spends any of the stored staking tx
// if so, it returns the found staking tx and the spent staking input index,
// otherwise, it returns nil and -1
func (si *StakingIndexer) getSpentStakingTx(tx *wire.MsgTx) (*indexerstore.StoredStakingTransaction, int) {
	for i, txIn := range tx.TxIn {
		maybeStakingTxHash := txIn.PreviousOutPoint.Hash
		stakingTx, err := si.GetStakingTxByHash(&maybeStakingTxHash)
		if err != nil {
			continue
		}

		// this ensures the spending tx spends the correct staking output
		if txIn.PreviousOutPoint.Index != stakingTx.StakingOutputIdx {
			continue
		}

		return stakingTx, i
	}

	return nil, -1
}

// getSpentStakingTx checks if the given tx spends any of the stored staking tx
// if so, it returns the found staking tx and the spent staking input index,
// otherwise, it returns nil and -1
func (si *StakingIndexer) getSpentUnbondingTx(tx *wire.MsgTx) (*indexerstore.StoredUnbondingTransaction, int) {
	for i, txIn := range tx.TxIn {
		maybeUnbondingTxHash := txIn.PreviousOutPoint.Hash
		unbondingTx, err := si.GetUnbondingTxByHash(&maybeUnbondingTxHash)
		if err != nil {
			continue
		}

		return unbondingTx, i
	}

	return nil, -1
}

// IsValidUnbondingTx tries to identify a tx is a valid unbonding tx
// It returns error when (1) it fails to verify the unbonding tx due
// to invalid parameters, and (2) the tx spends the unbonding path
// but is invalid
func (si *StakingIndexer) IsValidUnbondingTx(tx *wire.MsgTx, stakingTx *indexerstore.StoredStakingTransaction, params *types.GlobalParams) (bool, error) {
	// 1. an unbonding tx must have exactly one input and output
	if len(tx.TxIn) != 1 {
		return false, nil
	}
	if len(tx.TxOut) != 1 {
		return false, nil
	}

	// 2. an unbonding tx must spend the staking output
	stakingTxHash := stakingTx.Tx.TxHash()
	if !tx.TxIn[0].PreviousOutPoint.Hash.IsEqual(&stakingTxHash) {
		return false, nil
	}
	if tx.TxIn[0].PreviousOutPoint.Index != stakingTx.StakingOutputIdx {
		return false, nil
	}

	// 3. the script of an unbonding tx output must be expected
	// as re-built unbonding output from params
	stakingValue := btcutil.Amount(stakingTx.Tx.TxOut[stakingTx.StakingOutputIdx].Value)
	expectedUnbondingOutputValue := stakingValue - params.UnbondingFee
	if expectedUnbondingOutputValue <= 0 {
		return false, fmt.Errorf("%w: staking output value is too low, got %v, unbonding fee: %v",
			ErrInvalidUnbondingTx, stakingValue, params.UnbondingFee)
	}
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakingTx.StakerPk,
		[]*btcec.PublicKey{stakingTx.FinalityProviderPk},
		params.CovenantPks,
		params.CovenantQuorum,
		params.UnbondingTime,
		expectedUnbondingOutputValue,
		&si.cfg.BTCNetParams,
	)
	if err != nil {
		return false, fmt.Errorf("%w: failed to rebuid the unbonding info", ErrInvalidGlobalParameters)
	}
	if !bytes.Equal(tx.TxOut[0].PkScript, unbondingInfo.UnbondingOutput.PkScript) {
		// the tx does not spend the unbonding path, thus not an unbonding tx
		return false, nil
	}
	if tx.TxOut[0].Value != unbondingInfo.UnbondingOutput.Value {
		return false, fmt.Errorf("%w: the unbonding output value %d is not expected %d",
			ErrInvalidUnbondingTx, tx.TxOut[0].Value, unbondingInfo.UnbondingOutput.Value)
	}

	return true, nil
}

func (si *StakingIndexer) ProcessStakingTx(
	tx *wire.MsgTx,
	stakingData *btcstaking.ParsedV0StakingTx,
	height uint64, timestamp time.Time,
	params *types.GlobalParams,
) error {

	si.logger.Info("found a staking tx",
		zap.Uint64("height", height),
		zap.String("tx_hash", tx.TxHash().String()),
	)

	txHex, err := getTxHex(tx)
	if err != nil {
		return err
	}

	stakerPkHex := hex.EncodeToString(stakingData.OpReturnData.StakerPublicKey.Marshall())
	fpPkHex := hex.EncodeToString(stakingData.OpReturnData.FinalityProviderPublicKey.Marshall())

	// Step 1: Check against global parameters such as min/max staking amount and staking time
	validationErr := si.validateStakingTx(params, stakingData)
	if validationErr != nil {
		return validationErr
	}
	// Step 2: Overflow check (staking cap)
	isOverflow, err := si.isOverflow(uint64(params.StakingCap), uint64(stakingData.StakingOutput.Value))
	if err != nil {
		return fmt.Errorf("failed to check the overflow of staking tx: %w", err)
	}

	stakingEvent := queuecli.NewActiveStakingEvent(
		tx.TxHash().String(),
		stakerPkHex,
		fpPkHex,
		uint64(stakingData.StakingOutput.Value),
		height,
		timestamp.Unix(),
		uint64(stakingData.OpReturnData.StakingTime),
		uint64(stakingData.StakingOutputIdx),
		txHex,
		isOverflow,
	)

	// push the events first with the assumption that the consumer can handle duplicate events
	if err := si.consumer.PushStakingEvent(&stakingEvent); err != nil {
		return fmt.Errorf("failed to push the staking event to the consumer: %w", err)
	}

	txHashHex := tx.TxHash().String()
	si.logger.Info("successfully pushing the staking event",
		zap.String("tx_hash", txHashHex))

	if err := si.is.AddStakingTransaction(
		tx,
		uint32(stakingData.StakingOutputIdx),
		height,
		stakingData.OpReturnData.StakerPublicKey.PubKey,
		uint32(stakingData.OpReturnData.StakingTime),
		stakingData.OpReturnData.FinalityProviderPublicKey.PubKey,
		uint64(stakingData.StakingOutput.Value),
		isOverflow,
	); err != nil {
		return fmt.Errorf("failed to add the staking tx to store: %w", err)
	}

	si.logger.Info("successfully saving the staking tx",
		zap.String("tx_hash", txHashHex))

	// record metrics
	totalStakingTxs.Inc()
	lastFoundStakingTx.WithLabelValues(
		strconv.Itoa(int(height)),
		tx.TxHash().String(),
		stakerPkHex,
		strconv.Itoa(int(stakingData.StakingOutput.Value)),
		strconv.Itoa(int(stakingData.OpReturnData.StakingTime)),
		fpPkHex,
	).SetToCurrentTime()

	return nil
}

func (si *StakingIndexer) ProcessUnbondingTx(
	tx *wire.MsgTx,
	stakingTxHash *chainhash.Hash,
	height uint64, timestamp time.Time,
	params *types.GlobalParams,
) error {

	si.logger.Info("found an unbonding tx",
		zap.Uint64("height", height),
		zap.String("tx_hash", tx.TxHash().String()),
		zap.String("staking_tx_hash", stakingTxHash.String()),
	)

	txHex, err := getTxHex(tx)
	if err != nil {
		return err
	}

	unbondingEvent := queuecli.NewUnbondingStakingEvent(
		stakingTxHash.String(),
		height,
		timestamp.Unix(),
		uint64(params.UnbondingTime),
		// valid unbonding tx always has one output
		0,
		txHex,
		tx.TxHash().String(),
	)

	if err := si.consumer.PushUnbondingEvent(&unbondingEvent); err != nil {
		return fmt.Errorf("failed to push the unbonding event to the consumer: %w", err)
	}

	si.logger.Info("successfully pushing the unbonding event",
		zap.String("tx_hash", tx.TxHash().String()))

	if err := si.is.AddUnbondingTransaction(
		tx,
		stakingTxHash,
	); err != nil {
		return fmt.Errorf("failed to add the unbonding tx to store: %w", err)
	}

	si.logger.Info("successfully saving the unbonding tx",
		zap.String("tx_hash", tx.TxHash().String()))

	// record metrics
	totalUnbondingTxs.Inc()
	lastFoundUnbondingTx.WithLabelValues(
		strconv.Itoa(int(height)),
		tx.TxHash().String(),
		stakingTxHash.String(),
	).SetToCurrentTime()

	return nil
}

func (si *StakingIndexer) processWithdrawTx(tx *wire.MsgTx, stakingTxHash *chainhash.Hash, unbondingTxHash *chainhash.Hash, height uint64) error {
	txHashHex := tx.TxHash().String()
	if unbondingTxHash == nil {
		si.logger.Info("found a withdraw tx from staking",
			zap.String("tx_hash", txHashHex),
			zap.String("staking_tx_hash", stakingTxHash.String()),
		)
	} else {
		si.logger.Info("found a withdraw tx from unbonding",
			zap.String("tx_hash", txHashHex),
			zap.String("staking_tx_hash", stakingTxHash.String()),
			zap.String("unbonding_tx_hash", unbondingTxHash.String()),
		)
	}

	withdrawEvent := queuecli.NewWithdrawStakingEvent(stakingTxHash.String())

	if err := si.consumer.PushWithdrawEvent(&withdrawEvent); err != nil {
		return fmt.Errorf("failed to push the withdraw event to the consumer: %w", err)
	}

	si.logger.Info("successfully pushing the withdraw event",
		zap.String("tx_hash", txHashHex))

	// record metrics
	if unbondingTxHash == nil {
		totalWithdrawTxsFromStaking.Inc()
		lastFoundWithdrawTxFromStaking.WithLabelValues(
			strconv.Itoa(int(height)),
			txHashHex,
			stakingTxHash.String(),
		).SetToCurrentTime()
	} else {
		totalWithdrawTxsFromUnbonding.Inc()
		lastFoundWithdrawTxFromUnbonding.WithLabelValues(
			strconv.Itoa(int(height)),
			txHashHex,
			unbondingTxHash.String(),
			stakingTxHash.String(),
		).SetToCurrentTime()
	}

	return nil
}

func (si *StakingIndexer) tryParseStakingTx(tx *wire.MsgTx, params *types.GlobalParams) (*btcstaking.ParsedV0StakingTx, error) {
	possible := btcstaking.IsPossibleV0StakingTx(tx, params.Tag)
	if !possible {
		return nil, fmt.Errorf("not staking tx")
	}

	parsedData, err := btcstaking.ParseV0StakingTx(
		tx,
		params.Tag,
		params.CovenantPks,
		params.CovenantQuorum,
		&si.cfg.BTCNetParams)
	if err != nil {
		return nil, fmt.Errorf("not staking tx")
	}

	return parsedData, nil
}

func (si *StakingIndexer) GetStakingTxByHash(hash *chainhash.Hash) (*indexerstore.StoredStakingTransaction, error) {
	return si.is.GetStakingTransaction(hash)
}

func (si *StakingIndexer) GetUnbondingTxByHash(hash *chainhash.Hash) (*indexerstore.StoredUnbondingTransaction, error) {
	return si.is.GetUnbondingTransaction(hash)
}

func (si *StakingIndexer) Stop() error {
	var stopErr error
	si.stopOnce.Do(func() {
		si.logger.Info("Stopping Staking Indexer App")

		close(si.quit)
		si.wg.Wait()

		if err := si.btcScanner.Stop(); err != nil {
			stopErr = err
			return
		}

		si.logger.Info("Staking Indexer App is successfully stopped!")

	})
	return stopErr
}

func getTxHex(tx *wire.MsgTx) (string, error) {
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return "", fmt.Errorf("failed to serialize the tx: %w", err)
	}
	txHex := hex.EncodeToString(buf.Bytes())

	return txHex, nil
}

// validateStakingTx performs the validation checks for the staking tx
// such as min and max staking amount and staking time
func (si *StakingIndexer) validateStakingTx(params *types.GlobalParams, stakingData *btcstaking.ParsedV0StakingTx) error {
	value := stakingData.StakingOutput.Value
	// Minimum staking amount check
	if value < int64(params.MinStakingAmount) {
		return fmt.Errorf("%w: staking amount is too low, expected: %v, got: %v",
			ErrInvalidStakingTx, params.MinStakingAmount, value)
	}

	// Maximum staking amount check
	if value > int64(params.MaxStakingAmount) {
		return fmt.Errorf("%w: staking amount is too high, expected: %v, got: %v",
			ErrInvalidStakingTx, params.MaxStakingAmount, value)
	}

	// Maximum staking time check
	if uint64(stakingData.OpReturnData.StakingTime) > uint64(params.MaxStakingTime) {
		return fmt.Errorf("%w: staking time is too high, expected: %v, got: %v",
			ErrInvalidStakingTx, params.MaxStakingTime, stakingData.OpReturnData.StakingTime)
	}

	// Minimum staking time check
	if uint64(stakingData.OpReturnData.StakingTime) < uint64(params.MinStakingTime) {
		return fmt.Errorf("%w: staking time is too low, expected: %v, got: %v",
			ErrInvalidStakingTx, params.MinStakingTime, stakingData.OpReturnData.StakingTime)
	}
	return nil
}

func (si *StakingIndexer) isOverflow(cap uint64, stakingValue uint64) (bool, error) {
	confirmedTvl, err := si.is.GetConfirmedTvl()
	if err != nil {
		return false, fmt.Errorf("failed to get the confirmed TVL: %w", err)
	}

	return confirmedTvl+stakingValue > cap, nil
}

func (si *StakingIndexer) GetConfirmedTvl() (uint64, error) {
	return si.is.GetConfirmedTvl()
}
