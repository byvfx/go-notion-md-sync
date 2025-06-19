package cli

import (
	"fmt"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify configuration and readiness",
	Long: `Verify that notion-md-sync is properly configured and ready to use.

This command checks:
- Whether the configuration file is valid
- That all required settings are present
- The current parent page ID in Notion
- The markdown root directory
- Sync direction and conflict resolution settings`,
	RunE: runVerify,
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		// Configuration not found or invalid
		fmt.Println("❌ Configuration Status: NOT CONFIGURED")
		fmt.Printf("   Error: %v\n", err)
		fmt.Println("\nPlease create a configuration file to use notion-md-sync.")
		fmt.Println("See config.example.yaml for reference.")
		return nil
	}

	// Check configuration validity
	configValid := true
	var configIssues []string

	if cfg.Notion.Token == "" {
		configValid = false
		configIssues = append(configIssues, "Missing Notion API token")
	}
	if cfg.Notion.ParentPageID == "" {
		configValid = false
		configIssues = append(configIssues, "Missing parent page ID")
	}
	if cfg.Directories.MarkdownRoot == "" {
		configValid = false
		configIssues = append(configIssues, "Missing markdown root directory")
	}

	// Display configuration status
	if configValid {
		fmt.Println("✅ Configuration Status: READY")
		fmt.Printf("   Parent Page ID: %s\n", cfg.Notion.ParentPageID)
		fmt.Printf("   Markdown Root: %s\n", cfg.Directories.MarkdownRoot)
		fmt.Printf("   Sync Direction: %s\n", cfg.Sync.Direction)
		fmt.Printf("   Conflict Resolution: %s\n", cfg.Sync.ConflictResolution)
	} else {
		fmt.Println("❌ Configuration Status: INCOMPLETE")
		for _, issue := range configIssues {
			fmt.Printf("   - %s\n", issue)
		}
	}

	return nil
}
