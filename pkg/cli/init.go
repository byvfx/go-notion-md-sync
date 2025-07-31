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
	// Get current working directory for display
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = "current directory"
	}

	fmt.Println("🚀 Initializing notion-md-sync project...")
	fmt.Printf("📍 Working in: %s\n\n", currentDir)

	// Check if already initialized
	configPath := filepath.Join(currentDir, "config.yaml")
	if _, err := os.Stat("config.yaml"); err == nil {
		fmt.Printf("⚠️  Project already initialized!\n")
		fmt.Printf("📄 Found existing config: %s\n", configPath)
		
		// Show existing .env file location if it exists
		envPath := filepath.Join(currentDir, ".env")
		if _, err := os.Stat(".env"); err == nil {
			fmt.Printf("🔑 Found existing credentials: %s\n", envPath)
		} else {
			fmt.Printf("💡 You can create credentials at: %s\n", envPath)
		}
		
		fmt.Println("\n✅ Your project is ready to use!")
		fmt.Println("📚 Next steps:")
		fmt.Println("   • Run: notion-md-sync pull --verbose")
		fmt.Println("   • Or: notion-md-sync push --verbose")
		fmt.Println("   • Or: notion-md-sync --help")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("🔧 Let's set up your Notion integration...")
	fmt.Println()

	// Get Notion token
	var token string
	for {
		fmt.Println("🔑 Step 1: Notion Integration Token")
		fmt.Println("   Get yours at: https://www.notion.so/my-integrations")
		fmt.Println("   Create a new integration and copy the 'Internal Integration Token'")
		fmt.Print("\n📋 Paste your token here (you can copy/paste): ")
		token, _ = reader.ReadString('\n')
		token = strings.TrimSpace(token)

		if err := util.ValidateNotionToken(token); err != nil {
			fmt.Printf("❌ Invalid token: %v\n", err)
			fmt.Println("   💡 Make sure you copied the full token from Notion")
			continue
		}
		fmt.Println("✅ Valid token!")
		break
	}

	// Get parent page ID
	var pageID string
	for {
		fmt.Println("\n📄 Step 2: Parent Page ID")
		fmt.Println("   1. Open your Notion page in browser")
		fmt.Println("   2. Share the page with your integration")
		fmt.Println("   3. Copy the page ID from the URL (long string after last '/')")
		fmt.Print("\n📋 Paste your page ID here: ")
		pageID, _ = reader.ReadString('\n')
		pageID = strings.TrimSpace(pageID)

		if err := util.ValidateNotionPageID(pageID); err != nil {
			fmt.Printf("❌ Invalid page ID: %v\n", err)
			fmt.Println("   💡 Should be a 32-character string with dashes")
			continue
		}
		fmt.Println("✅ Valid page ID!")
		break
	}

	// Get markdown directory
	var markdownDir string
	for {
		fmt.Println("\n📂 Step 3: Markdown Directory")
		fmt.Println("   Where should we store your markdown files?")
		fmt.Print("   Directory path (default: ./docs): ")
		markdownDir, _ = reader.ReadString('\n')
		markdownDir = strings.TrimSpace(markdownDir)
		if markdownDir == "" {
			markdownDir = "./docs"
		}

		if err := util.ValidateDirectoryPath(markdownDir, false); err != nil {
			fmt.Printf("❌ Invalid directory path: %v\n", err)
			continue
		}
		fmt.Printf("✅ Will create directory: %s\n", markdownDir)
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

# Performance optimization settings
# Based on extensive testing showing 26%% performance improvement
performance:
  # Worker count: 0 = auto-detect (recommended)
  # - Small workspaces (<5 pages): Uses page count
  # - Medium workspaces (5-14 pages): Uses 20 workers
  # - Large workspaces (15+ pages): Uses 30 workers
  workers: 0
  
  # Multi-client mode (experimental)
  # Standard single client usually performs best
  use_multi_client: false
  
  # Number of HTTP clients when multi-client is enabled
  client_count: 3
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

	// Success message with full paths
	configFullPath := filepath.Join(currentDir, "config.yaml")
	envFullPath := filepath.Join(currentDir, ".env")
	envExampleFullPath := filepath.Join(currentDir, ".env.example")
	sampleFullPath := filepath.Join(currentDir, samplePath)
	markdownFullPath := filepath.Join(currentDir, markdownDir)

	fmt.Println("\n✅ Project initialized successfully!")
	fmt.Println("\n📁 Created files:")
	fmt.Printf("   📄 %s (configuration)\n", configFullPath)
	fmt.Printf("   🔑 %s (your credentials)\n", envFullPath)
	fmt.Printf("   📋 %s (template)\n", envExampleFullPath)
	fmt.Printf("   📝 %s (sample markdown)\n", sampleFullPath)
	fmt.Printf("   📂 %s/ (markdown directory)\n", markdownFullPath)

	fmt.Println("\n💡 Your credentials are ready!")
	fmt.Println("   Your Notion token and page ID have been saved to .env")
	fmt.Printf("   You can edit them anytime at: %s\n", envFullPath)

	fmt.Println("\n🚀 Ready to sync!")
	fmt.Println("   • Test connection: notion-md-sync pull --verbose")
	fmt.Println("   • Push sample file: notion-md-sync push --verbose")  
	fmt.Println("   • Auto-sync changes: notion-md-sync watch")

	fmt.Println("\n📚 Need help?")
	fmt.Println("   • All commands: notion-md-sync --help")
	fmt.Println("   • Documentation: https://github.com/byvfx/go-notion-md-sync")

	return nil
}
