package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/spf13/cobra"
)

var databaseCmd = &cobra.Command{
	Use:   "database",
	Short: "Sync between Notion databases and CSV files",
	Long:  `Commands for syncing data between Notion databases and CSV files`,
}

var dbExportCmd = &cobra.Command{
	Use:   "export <database-id> <csv-file>",
	Short: "Export Notion database to CSV file",
	Long:  `Export all data from a Notion database to a CSV file`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		databaseID := args[0]
		csvPath := args[1]

		// Make path absolute
		csvPath, err := filepath.Abs(csvPath)
		if err != nil {
			return fmt.Errorf("failed to resolve CSV path: %w", err)
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client := notion.NewClient(cfg.Notion.Token)
		dbSync := sync.NewDatabaseSync(client)

		ctx := context.Background()
		fmt.Printf("Exporting database %s to %s...\n", databaseID, csvPath)

		err = dbSync.SyncNotionDatabaseToCSV(ctx, databaseID, csvPath)
		if err != nil {
			return fmt.Errorf("failed to export database: %w", err)
		}

		fmt.Printf("✓ Successfully exported database to %s\n", csvPath)
		return nil
	},
}

var dbImportCmd = &cobra.Command{
	Use:   "import <csv-file> <database-id>",
	Short: "Import CSV file to existing Notion database",
	Long:  `Import data from a CSV file to an existing Notion database`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		csvPath := args[0]
		databaseID := args[1]

		// Make path absolute
		csvPath, err := filepath.Abs(csvPath)
		if err != nil {
			return fmt.Errorf("failed to resolve CSV path: %w", err)
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client := notion.NewClient(cfg.Notion.Token)
		dbSync := sync.NewDatabaseSync(client)

		ctx := context.Background()
		fmt.Printf("Importing %s to database %s...\n", csvPath, databaseID)

		err = dbSync.SyncCSVToNotionDatabase(ctx, csvPath, databaseID)
		if err != nil {
			return fmt.Errorf("failed to import CSV: %w", err)
		}

		fmt.Printf("✓ Successfully imported CSV to database\n")
		return nil
	},
}

var dbCreateCmd = &cobra.Command{
	Use:   "create <csv-file> <parent-page-id>",
	Short: "Create new Notion database from CSV file",
	Long:  `Create a new Notion database and import data from a CSV file`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		csvPath := args[0]
		parentPageID := args[1]

		// Make path absolute
		csvPath, err := filepath.Abs(csvPath)
		if err != nil {
			return fmt.Errorf("failed to resolve CSV path: %w", err)
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client := notion.NewClient(cfg.Notion.Token)
		dbSync := sync.NewDatabaseSync(client)

		ctx := context.Background()
		fmt.Printf("Creating new database from %s in page %s...\n", csvPath, parentPageID)

		database, err := dbSync.CreateDatabaseFromCSV(ctx, csvPath, parentPageID)
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}

		fmt.Printf("✓ Successfully created database: %s\n", database.ID)
		fmt.Printf("  URL: %s\n", database.URL)
		return nil
	},
}

func init() {
	databaseCmd.AddCommand(dbExportCmd)
	databaseCmd.AddCommand(dbImportCmd)
	databaseCmd.AddCommand(dbCreateCmd)
}
