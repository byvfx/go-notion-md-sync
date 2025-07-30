package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/byvfx/go-notion-md-sync/pkg/util"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new notion-md-sync project",
	Long: `Initialize creates a new notion-md-sync project in the current directory.
It will create the necessary configuration files and directories to get you started.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Initializing notion-md-sync project...")

	// Check if already initialized
	if _, err := os.Stat("config.yaml"); err == nil {
		fmt.Println("⚠️  Project already initialized (config.yaml exists)")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	// Get Notion token
	var token string
	for {
		fmt.Print("Enter your Notion Integration Token: ")
		token, _ = reader.ReadString('\n')
		token = strings.TrimSpace(token)

		if err := util.ValidateNotionToken(token); err != nil {
			fmt.Printf("❌ Invalid token: %v\n", err)
			continue
		}
		break
	}

	// Get parent page ID
	var pageID string
	for {
		fmt.Print("Enter your Notion Parent Page ID: ")
		pageID, _ = reader.ReadString('\n')
		pageID = strings.TrimSpace(pageID)

		if err := util.ValidateNotionPageID(pageID); err != nil {
			fmt.Printf("❌ Invalid page ID: %v\n", err)
			continue
		}
		break
	}

	// Get markdown directory
	var markdownDir string
	for {
		fmt.Print("Markdown directory (default: ./docs): ")
		markdownDir, _ = reader.ReadString('\n')
		markdownDir = strings.TrimSpace(markdownDir)
		if markdownDir == "" {
			markdownDir = "./docs"
		}

		if err := util.ValidateDirectoryPath(markdownDir, false); err != nil {
			fmt.Printf("❌ Invalid directory path: %v\n", err)
			continue
		}
		break
	}

	// Create directories
	if err := os.MkdirAll(markdownDir, 0755); err != nil {
		return fmt.Errorf("failed to create markdown directory: %w", err)
	}

	// Create config.yaml
	configContent := fmt.Sprintf(`# notion-md-sync configuration
notion:
  token: ""  # Set via NOTION_MD_SYNC_NOTION_TOKEN environment variable
  parent_page_id: ""  # Set via NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID environment variable

sync:
  direction: push
  conflict_resolution: newer

directories:
  markdown_root: %s
  excluded_patterns:
    - "*.tmp"
    - "node_modules/**"
    - ".git/**"

mapping:
  strategy: frontmatter
`, markdownDir)

	if err := os.WriteFile("config.yaml", []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to create config.yaml: %w", err)
	}

	// Create .env file
	envContent := fmt.Sprintf(`# notion-md-sync environment variables
# Get your token from: https://www.notion.so/my-integrations
NOTION_MD_SYNC_NOTION_TOKEN=%s

# Get your page ID from the Notion page URL (the long string after the last /)
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=%s
`, token, pageID)

	if err := os.WriteFile(".env", []byte(envContent), 0644); err != nil {
		return fmt.Errorf("failed to create .env: %w", err)
	}

	// Create .env.example
	envExampleContent := `# Copy this file to .env and fill in your actual values
NOTION_MD_SYNC_NOTION_TOKEN=your_integration_token_here
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=your_parent_page_id_here
`

	if err := os.WriteFile(".env.example", []byte(envExampleContent), 0644); err != nil {
		return fmt.Errorf("failed to create .env.example: %w", err)
	}

	// Create sample markdown file
	sampleContent := `---
title: "Welcome to notion-md-sync"
sync_enabled: true
---

# Welcome to notion-md-sync

This is a sample markdown file that demonstrates how notion-md-sync works.

## Getting Started

1. Edit this file
2. Run: notion-md-sync push
3. Check your Notion page!

## Features

- **Bidirectional sync** between markdown and Notion
- **Frontmatter support** for metadata
- **File watching** for automatic sync
- **Flexible configuration**

Happy syncing! 🚀
`

	samplePath := filepath.Join(markdownDir, "welcome.md")
	if err := os.WriteFile(samplePath, []byte(sampleContent), 0644); err != nil {
		return fmt.Errorf("failed to create sample file: %w", err)
	}

	// Success message
	fmt.Println("\n✅ Project initialized successfully!")
	fmt.Println("\n📁 Created files:")
	fmt.Println("   - config.yaml (configuration)")
	fmt.Println("   - .env (your credentials)")
	fmt.Println("   - .env.example (template)")
	fmt.Printf("   - %s (sample markdown)\n", samplePath)
	fmt.Printf("   - %s/ (markdown directory)\n", markdownDir)

	fmt.Println("\n🔑 Next steps:")
	fmt.Println("   1. Edit .env with your actual Notion credentials")
	fmt.Println("   2. Test with: notion-md-sync push --verbose")
	fmt.Println("   3. Start syncing: notion-md-sync watch")

	fmt.Println("\n📚 Need help?")
	fmt.Println("   - Run: notion-md-sync --help")
	fmt.Println("   - Visit: https://github.com/byvfx/go-notion-md-sync")

	return nil
}
