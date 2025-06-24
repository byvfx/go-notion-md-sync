package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestSyncCommand_ArgumentParsing(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "invalid direction",
			args:        []string{"invalid", "--config", "test.yaml"},
			wantErr:     true,
			errContains: "invalid direction: invalid",
		},
		{
			name:    "valid push direction",
			args:    []string{"push", "--config", "test.yaml"},
			wantErr: true, // Will fail because config doesn't exist, but that's OK
			errContains: "failed to load config",
		},
		{
			name:    "valid pull direction", 
			args:    []string{"pull", "--config", "test.yaml"},
			wantErr: true, // Will fail because config doesn't exist, but that's OK
			errContains: "failed to load config",
		},
		{
			name:    "no args uses default direction",
			args:    []string{"--config", "test.yaml"},
			wantErr: true, // Will fail because config doesn't exist, but that's OK
			errContains: "failed to load config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := &cobra.Command{
				Use:   "sync [direction]",
				Short: "Sync between markdown and Notion",
				Args:  cobra.MaximumNArgs(1),
				RunE:  runSync,
			}

			// Reset global flags
			syncFile = ""
			syncDirection = "push"
			syncDirectory = ""
			dryRun = false
			configPath = ""

			// Add flags
			cmd.Flags().StringVarP(&syncFile, "file", "f", "", "specific file to sync")
			cmd.Flags().StringVarP(&syncDirection, "direction", "d", "push", "sync direction")
			cmd.Flags().StringVar(&syncDirectory, "directory", "", "directory containing markdown files")
			cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be synced")
			cmd.Flags().StringVarP(&configPath, "config", "c", "", "config file path")

			// Capture output
			output := &bytes.Buffer{}
			cmd.SetOut(output)
			cmd.SetErr(output)
			cmd.SetArgs(tt.args)

			// Execute
			err := cmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}