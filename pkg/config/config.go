package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

type Config struct {
	Notion struct {
		Token        string `yaml:"token" mapstructure:"token"`
		ParentPageID string `yaml:"parent_page_id" mapstructure:"parent_page_id"`
	} `yaml:"notion" mapstructure:"notion"`

	Sync struct {
		Direction          string `yaml:"direction" mapstructure:"direction"`
		ConflictResolution string `yaml:"conflict_resolution" mapstructure:"conflict_resolution"`
	} `yaml:"sync" mapstructure:"sync"`

	Directories struct {
		MarkdownRoot     string   `yaml:"markdown_root" mapstructure:"markdown_root"`
		ExcludedPatterns []string `yaml:"excluded_patterns" mapstructure:"excluded_patterns"`
	} `yaml:"directories" mapstructure:"directories"`

	Mapping struct {
		Strategy string `yaml:"strategy" mapstructure:"strategy"`
	} `yaml:"mapping" mapstructure:"mapping"`
}

func Load(configPath string) (*Config, error) {
	// Load .env file if it exists
	loadEnvFile()

	v := viper.New()

	// Set defaults
	v.SetDefault("sync.direction", "push")
	v.SetDefault("sync.conflict_resolution", "newer")
	v.SetDefault("directories.markdown_root", "./")
	v.SetDefault("mapping.strategy", "filename")

	// Environment variable support
	v.SetEnvPrefix("NOTION_MD_SYNC")
	v.AutomaticEnv()

	// Bind specific environment variables for nested config
	v.BindEnv("notion.token", "NOTION_MD_SYNC_NOTION_TOKEN")
	v.BindEnv("notion.parent_page_id", "NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID")

	// Config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Look for config in working directory and home directory
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")

		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".notion-md-sync"))
		}
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if config.Notion.Token == "" {
		return nil, fmt.Errorf("notion.token is required")
	}
	if config.Notion.ParentPageID == "" {
		return nil, fmt.Errorf("notion.parent_page_id is required")
	}

	return &config, nil
}

// loadEnvFile loads .env file from current directory or parent directories
func loadEnvFile() {
	// Try to load .env from current directory first
	if err := gotenv.Load(".env"); err == nil {
		return
	}

	// Try to load from home directory
	if home, err := os.UserHomeDir(); err == nil {
		envPath := filepath.Join(home, ".notion-md-sync", ".env")
		if err := gotenv.Load(envPath); err == nil {
			return
		}
	}

	// Try to find .env in parent directories (walk up to 3 levels)
	currentDir, err := os.Getwd()
	if err != nil {
		return
	}

	for i := 0; i < 3; i++ {
		envPath := filepath.Join(currentDir, ".env")
		if err := gotenv.Load(envPath); err == nil {
			return
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break // reached root
		}
		currentDir = parentDir
	}
}
