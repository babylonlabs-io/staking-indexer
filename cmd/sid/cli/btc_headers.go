package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	babylontypes "github.com/babylonlabs-io/babylon/types"
	bbnbtclightclienttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/urfave/cli"
	"go.uber.org/zap"

	"github.com/babylonlabs-io/staking-indexer/btcclient"
	"github.com/babylonlabs-io/staking-indexer/btcscanner"
	"github.com/babylonlabs-io/staking-indexer/config"
	"github.com/babylonlabs-io/staking-indexer/log"
	"github.com/babylonlabs-io/staking-indexer/utils"
)

const (
	outputFileFlag        = "output"
	withHeightFlag        = "with-height"
	defaultOutputFileName = "btc-headers.json"
	filePermission        = 0600
)

type HeadersState struct {
	BtcHeaders []*bbnbtclightclienttypes.BTCHeaderInfo `json:"btc_headers,omitempty"`
}

var BtcHeaderCommand = cli.Command{
	Name:        "btc-headers",
	Usage:       "Output a range of BTC headers into a JSON file.",
	Description: "Output a range of BTC headers into a JSON file.",
	UsageText:   fmt.Sprintf("btc-headers [from] [to] [--%s=path/to/btc-headers.json]", outputFileFlag),
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "The path to the staking indexer home directory",
			Value: config.DefaultHomeDir,
		},
		cli.StringFlag{
			Name:  outputFileFlag,
			Usage: "The path to the output file",
			Value: filepath.Join(config.DefaultHomeDir, defaultOutputFileName),
		},
		cli.BoolFlag{
			Name:  withHeightFlag,
			Usage: "If it should fill the BTC block height property",
		},
	},
	Action: btcHeaders,
}

func btcHeaders(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 2 {
		return fmt.Errorf("not enough params, please specify [from] and [to]")
	}

	fromStr, toStr := args[0], args[1]
	fromBlock, err := strconv.ParseUint(fromStr, 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse %s: %w", fromStr, err)
	}

	toBlock, err := strconv.ParseUint(toStr, 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse %s: %w", toStr, err)
	}

	if fromBlock > toBlock {
		return fmt.Errorf("the [from] %d should not be greater than the [to] %d", fromBlock, toBlock)
	}

	homePath, err := filepath.Abs(ctx.String(homeFlag))
	if err != nil {
		return err
	}
	homePath = utils.CleanAndExpandPath(homePath)

	cfg, err := config.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logger, err := log.NewRootLoggerWithFile(config.LogFile(homePath), cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize the logger: %w", err)
	}

	btcClient, err := btcclient.NewBTCClient(
		cfg.BTCConfig,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize the BTC client: %w", err)
	}

	btcHeaders, err := BtcHeaderInfoList(btcClient, fromBlock, toBlock, ctx.Bool(withHeightFlag))
	if err != nil {
		return fmt.Errorf("failed to get BTC headers: %w", err)
	}

	headersState := HeadersState{
		BtcHeaders: btcHeaders,
	}

	bz, err := json.MarshalIndent(headersState, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to generate json to set to output file %+v: %w", headersState, err)
	}

	outputFilePath := ctx.String(outputFileFlag)
	if err := os.WriteFile(outputFilePath, bz, filePermission); err != nil {
		return fmt.Errorf("failed to write to output file %s: %w", outputFilePath, err)
	}

	logger.Info(
		"Successfully wrote btc headers to file",
		zap.Uint64("fromBlock", fromBlock),
		zap.Uint64("toBlock", toBlock),
		zap.String("outputFile", outputFilePath),
	)
	return nil
}

// BtcHeaderInfoList queries the btc client for (fromBlk ~ toBlk) BTC blocks, converting to BTCHeaderInfo.
func BtcHeaderInfoList(btcClient btcscanner.Client, fromBlk, toBlk uint64, withHeight bool) ([]*bbnbtclightclienttypes.BTCHeaderInfo, error) {
	btcHeaders := make([]*bbnbtclightclienttypes.BTCHeaderInfo, 0, toBlk-fromBlk+1)

	for blkHeight := fromBlk; blkHeight <= toBlk; blkHeight++ {
		blkHeader, err := btcClient.GetBlockHeaderByHeight(blkHeight)
		if err != nil {
			return nil, fmt.Errorf("failed to get block height %d from BTC client: %w", blkHeight, err)
		}

		headerBytes := babylontypes.NewBTCHeaderBytesFromBlockHeader(blkHeader)
		info := &bbnbtclightclienttypes.BTCHeaderInfo{
			Header: &headerBytes,
		}

		if withHeight {
			info.Height = uint32(blkHeight)
		}

		btcHeaders = append(btcHeaders, info)
	}
	return btcHeaders, nil
}
