package main

import (
	"fmt"
	"path/filepath"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/urfave/cli"

	"github.com/babylonchain/staking-indexer/btcscanner"
	"github.com/babylonchain/staking-indexer/config"
	"github.com/babylonchain/staking-indexer/consumer"
	"github.com/babylonchain/staking-indexer/indexer"
	"github.com/babylonchain/staking-indexer/indexerstore"
	"github.com/babylonchain/staking-indexer/log"
	"github.com/babylonchain/staking-indexer/params"
	service "github.com/babylonchain/staking-indexer/server"
	"github.com/babylonchain/staking-indexer/utils"
)

const (
	homeFlag        = "home"
	startHeightFlag = "start-height"
	paramsPathFlag  = "params-path"
)

var startCommand = cli.Command{
	Name:        "start",
	Usage:       "Start the staking-indexer server",
	Description: "Start the staking-indexer server.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "The path to the staking indexer home directory",
			Value: config.DefaultHomeDir,
		},
		cli.StringFlag{
			Name:  startHeightFlag,
			Usage: "The BTC height that the staking indexer starts from",
		},
		cli.StringFlag{
			Name:  paramsPathFlag,
			Usage: "The path to the global params file",
			Value: config.DefaultParamsPath,
		},
	},
	Action: start,
}

func start(ctx *cli.Context) error {
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

	// create BTC client and connect to BTC server
	btcClient, err := btcclient.NewWithBlockSubscriber(
		cfg.BTCConfig.ToVigilanteBTCConfig(),
		cfg.BTCConfig.RetrySleepTime,
		cfg.BTCConfig.MaxRetrySleepTime,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize the BTC client: %w", err)
	}

	btcNotifier, err := btcclient.NewNodeBackend(
		cfg.BTCConfig.ToBtcNodeBackendConfig(),
		&cfg.BTCNetParams,
		&btcclient.EmptyHintCache{},
	)
	if err != nil {
		return fmt.Errorf("failed to initialize the BTC notifier: %w", err)
	}

	dbBackend, err := cfg.DatabaseConfig.GetDbBackend()
	if err != nil {
		return fmt.Errorf("failed to create db backend: %w", err)
	}

	is, err := indexerstore.NewIndexerStore(dbBackend)
	if err != nil {
		return fmt.Errorf("failed to initiate staking indexer store: %w", err)
	}

	var startHeight uint64
	lastProcessedHeight, err := is.GetLastProcessedHeight()
	if err != nil && !ctx.IsSet(startHeightFlag) {
		return fmt.Errorf("should specify %s", startHeightFlag)
	}
	if ctx.IsSet(startHeightFlag) {
		startHeight = ctx.Uint64(startHeightFlag)
	} else {
		startHeight = lastProcessedHeight + 1
	}

	// create BTC scanner
	scanner, err := btcscanner.NewBTCScanner(cfg.BTCScannerConfig, logger, btcClient, btcNotifier, startHeight)
	if err != nil {
		return fmt.Errorf("failed to initialize the BTC scanner: %w", err)
	}

	// create event consumer
	queueConsumer, err := consumer.NewQueueConsumer(cfg.QueueConfig, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize event consumer: %w", err)
	}

	paramsRetriever, err := params.NewLocalParamsRetriever(ctx.String(paramsPathFlag))
	if err != nil {
		return fmt.Errorf("failed to initialize params retriever: %w", err)
	}

	// create the staking indexer app
	si, err := indexer.NewStakingIndexer(cfg, logger, queueConsumer, is, paramsRetriever.GetParams(), scanner.ConfirmedBlocksChan())
	if err != nil {
		return fmt.Errorf("failed to initialize the staking indexer app: %w", err)
	}

	// hook interceptor for os signals
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		return err
	}

	// create the server
	indexerServer := service.NewStakingIndexerServer(cfg, queueConsumer, dbBackend, btcNotifier, scanner, si, logger, shutdownInterceptor)

	// run all the services until shutdown
	return indexerServer.RunUntilShutdown()
}
