package cli

import (
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/urfave/cli"

	"github.com/babylonlabs-io/staking-indexer/config"
	"github.com/babylonlabs-io/staking-indexer/indexerstore"
	"github.com/babylonlabs-io/staking-indexer/utils"
)

const (
	defaultTxExportOutputFileName = "transactions.csv"
)

var ExportCommand = cli.Command{
	Name:      "export",
	Usage:     "Export transactions from the indexer store to a CSV file based on block height.",
	UsageText: fmt.Sprintf("export [start-height] [end-height] [--%s=path/to/%s]", defaultTxExportOutputFileName, outputFileFlag),
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "The path to the staking indexer home directory",
			Value: config.DefaultHomeDir,
		},
		cli.StringFlag{
			Name:  outputFileFlag,
			Usage: "Path to the export file",
			Value: filepath.Join(config.DefaultHomeDir, defaultTxExportOutputFileName),
		},
	},
	Action: exportTransactions,
}

func exportTransactions(c *cli.Context) error {
	args := c.Args()
	if len(args) != 2 {
		return fmt.Errorf("not enough params, please specify [start-height] and [end-height]")
	}

	startHeightStr, endHeightStr := args[0], args[1]
	startHeight, err := strconv.ParseUint(startHeightStr, 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse %s: %w", startHeightStr, err)
	}

	endHeight, err := strconv.ParseUint(endHeightStr, 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse %s: %w", endHeightStr, err)
	}

	if startHeight > endHeight {
		return fmt.Errorf("the [start-height] %d should not be greater than the [end-height] %d", startHeight, endHeight)
	}

	homePath, err := filepath.Abs(c.String(homeFlag))
	if err != nil {
		return err
	}
	homePath = utils.CleanAndExpandPath(homePath)

	outputPath := c.String("output")
	outputPath = utils.CleanAndExpandPath(outputPath)

	// Load configuration
	cfg, err := config.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize IndexerStore
	dbBackend, err := cfg.DatabaseConfig.GetDbBackend()
	if err != nil {
		return fmt.Errorf("failed to create db backend: %w", err)
	}
	defer dbBackend.Close()

	indexerStore, err := indexerstore.NewIndexerStore(dbBackend)
	if err != nil {
		return fmt.Errorf("failed to initialize IndexerStore: %w", err)
	}

	// Open output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	err = writer.Write([]string{"Transaction Hash", "Staking Output Index", "Inclusion Height", "Staker Public Key", "Staking Time", "Finality Provider Public Key", "Is Overflow", "Staking Value"})
	if err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Get start height and end height parameters

	fmt.Printf("Exporting transactions from height %d to %d\n", startHeight, endHeight)

	// Export data using ScanStoredStakingTransactions method
	err = indexerStore.ScanStoredStakingTransactions(func(tx *indexerstore.StoredStakingTransaction) error {

		// Filter based on height parameters
		if tx.InclusionHeight >= startHeight && tx.InclusionHeight < endHeight {
			fmt.Printf("Exporting transaction %s, InclusionHeight %d\n", tx.Tx.TxHash().String(), tx.InclusionHeight)
			record := []string{
				tx.Tx.TxHash().String(),
				fmt.Sprintf("%d", tx.StakingOutputIdx),
				fmt.Sprintf("%d", tx.InclusionHeight),
				hex.EncodeToString(schnorr.SerializePubKey(tx.StakerPk)),
				fmt.Sprintf("%d", tx.StakingTime),
				hex.EncodeToString(schnorr.SerializePubKey(tx.FinalityProviderPk)),
				fmt.Sprintf("%t", tx.IsOverflow),
				fmt.Sprintf("%d", tx.StakingValue),
			}
			return writer.Write(record)
		}
		return nil

	})

	if err != nil {
		return fmt.Errorf("failed to export transactions: %w", err)
	}

	fmt.Printf("Exporting transactions from %s to %s\n", homePath, outputPath)

	return nil
}
