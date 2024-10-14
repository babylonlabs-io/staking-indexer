package cli

import (
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/urfave/cli"

	"github.com/babylonlabs-io/staking-indexer/config"
	"github.com/babylonlabs-io/staking-indexer/indexerstore"
	"github.com/babylonlabs-io/staking-indexer/utils"
)

var ExportCommand = cli.Command{
	Name:  "export",
	Usage: "Export transactions from the indexer store to a CSV file based on block height.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "Path to the indexer home directory",
			Value: config.DefaultHomeDir,
		},
		cli.Uint64Flag{
			Name:  "start-height",
			Usage: "Start block height for exported transactions",
			Value: 0,
		},
		cli.Uint64Flag{
			Name:  "end-height",
			Usage: "End block height for exported transactions",
			Value: ^uint64(0),
		},
		cli.StringFlag{
			Name:  "output",
			Usage: "Path to the export file",
			Value: "transactions.csv",
		},
	},
	Action: exportTransactions,
}

func exportTransactions(c *cli.Context) error {
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
	startHeight := c.Uint64("start-height")
	endHeight := c.Uint64("end-height")

	fmt.Printf("Exporting transactions from height %d to %d\n", startHeight, endHeight)

	// Export data using ScanStoredStakingTransactions method
	err = indexerStore.ScanStoredStakingTransactions(func(tx *indexerstore.StoredStakingTransaction) error {
		fmt.Printf("Exporting transaction %s, InclusionHeight %d\n", tx.Tx.TxHash().String(), tx.InclusionHeight)

		// Filter based on height parameters
		if tx.InclusionHeight >= startHeight && tx.InclusionHeight < endHeight {
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
		} else {
			return nil
		}
	})

	if err != nil {
		return fmt.Errorf("failed to export transactions: %w", err)
	}

	fmt.Printf("Exporting transactions from %s to %s\n", homePath, outputPath)

	return nil
}
