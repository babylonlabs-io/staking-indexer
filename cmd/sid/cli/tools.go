package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/urfave/cli"

	"github.com/babylonlabs-io/staking-indexer/config"
	"github.com/babylonlabs-io/staking-indexer/indexerstore"
	"github.com/babylonlabs-io/staking-indexer/utils"
)

var DbDumpCmd = cli.Command{
	Name:        "dump",
	Usage:       "Dump the staking data from db",
	Description: "Dump the staking data from db.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "The path to the staking indexer home directory",
			Value: config.DefaultHomeDir,
		},
	},
	Action: dumpDb,
}

func dumpDb(ctx *cli.Context) error {
	homePath, err := filepath.Abs(ctx.String(homeFlag))
	if err != nil {
		return err
	}
	homePath = utils.CleanAndExpandPath(homePath)

	cfg, err := config.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	dbBackend, err := cfg.DatabaseConfig.GetDbBackend()
	if err != nil {
		return fmt.Errorf("failed to create db backend: %w", err)
	}

	is, err := indexerstore.NewIndexerStore(dbBackend)
	if err != nil {
		return fmt.Errorf("failed to initiate staking indexer store: %w", err)
	}

	stakingTxs, err := is.DumpStakingTransactions()
	if err != nil {
		return fmt.Errorf("failed to dump data from db: %w", err)
	}

	printJSON(stakingTxs)

	return nil
}

func printJSON(resp interface{}) {
	jsonBytes, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		fmt.Printf("unable to decode: %s", err.Error())
		return
	}

	fmt.Printf("%s\n", jsonBytes)
}
