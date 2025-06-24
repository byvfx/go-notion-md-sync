package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		checkOutput    func(t *testing.T, output string)
		checkErrOutput func(t *testing.T, errOutput string)
	}{
		{
			name:    "no args shows help",
			args:    []string{},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "notion-md-sync is a CLI tool")
				assert.Contains(t, output, "Usage:")
				assert.Contains(t, output, "Available Commands:")
			},
		},
		{
			name:    "help flag",
			args:    []string{"--help"},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "notion-md-sync is a CLI tool")
				assert.Contains(t, output, "Flags:")
			},
		},
		{
			name:    "version flag",
			args:    []string{"--version"},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "notion-md-sync version 1.0.0")
			},
		},
		{
			name:    "invalid command",
			args:    []string{"invalid-command"},
			wantErr: true,
			checkErrOutput: func(t *testing.T, errOutput string) {
				assert.Contains(t, errOutput, "unknown command")
			},
		},
		{
			name:    "config flag",
			args:    []string{"--config", "test.yaml", "--help"},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				// Should show help with config flag
				assert.Contains(t, output, "config file path")
			},
		},
		{
			name:    "verbose flag",
			args:    []string{"--verbose", "--help"},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				// Should show help with verbose flag
				assert.Contains(t, output, "verbose output")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset command for each test
			rootCmd = &cobra.Command{
				Use:     "notion-md-sync",
				Short:   "Bridge between markdown files and Notion pages",
				Long:    `notion-md-sync is a CLI tool that synchronizes markdown files with Notion pages.` + "\n" + `It supports bidirectional synchronization, allowing you to push changes from markdown` + "\n" + `to Notion or pull changes from Notion to markdown files.`,
				Version: "1.0.0",
			}

			// Re-initialize
			rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")
			rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

			// Add subcommands
			rootCmd.AddCommand(syncCmd)
			rootCmd.AddCommand(pullCmd)
			rootCmd.AddCommand(pushCmd)
			rootCmd.AddCommand(watchCmd)

			// Capture output
			output := &bytes.Buffer{}
			errOutput := &bytes.Buffer{}
			rootCmd.SetOut(output)
			rootCmd.SetErr(errOutput)
			rootCmd.SetArgs(tt.args)

			err := rootCmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, output.String())
			}

			if tt.checkErrOutput != nil {
				tt.checkErrOutput(t, errOutput.String())
			}
		})
	}
}

func TestExecute(t *testing.T) {
	// Save original command
	originalCmd := rootCmd
	defer func() { rootCmd = originalCmd }()

	// Create a test command that returns an error
	testErr := errors.New("test error")
	rootCmd = &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return testErr
		},
	}

	err := Execute()
	assert.Error(t, err)
	assert.Equal(t, testErr, err)
}

func TestPrintVerbose(t *testing.T) {
	tests := []struct {
		name       string
		verbose    bool
		format     string
		args       []interface{}
		wantOutput string
	}{
		{
			name:       "verbose enabled",
			verbose:    true,
			format:     "test message %s",
			args:       []interface{}{"arg1"},
			wantOutput: "[VERBOSE] test message arg1\n",
		},
		{
			name:       "verbose disabled",
			verbose:    false,
			format:     "test message",
			args:       []interface{}{},
			wantOutput: "",
		},
		{
			name:       "verbose with multiple args",
			verbose:    true,
			format:     "test %s %d %v",
			args:       []interface{}{"string", 42, true},
			wantOutput: "[VERBOSE] test string 42 true\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original stderr
			oldStderr := os.Stderr
			defer func() { os.Stderr = oldStderr }()

			// Create a pipe to capture stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Set verbose flag
			originalVerbose := verbose
			verbose = tt.verbose
			defer func() { verbose = originalVerbose }()

			// Call printVerbose
			printVerbose(tt.format, tt.args...)

			// Close writer and read output
			w.Close()
			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			output := string(buf[:n])

			assert.Equal(t, tt.wantOutput, output)
		})
	}
}

func TestCommandStructure(t *testing.T) {
	// Test that all expected subcommands are registered
	expectedCommands := []string{"sync", "pull", "push", "watch"}

	commands := rootCmd.Commands()
	commandNames := make([]string, 0, len(commands))
	for _, cmd := range commands {
		// Extract just the command name without arguments
		name := strings.Fields(cmd.Use)[0]
		commandNames = append(commandNames, name)
	}

	for _, expected := range expectedCommands {
		assert.Contains(t, commandNames, expected, "Command %s should be registered", expected)
	}
}

func TestPersistentFlags(t *testing.T) {
	// Test that persistent flags are properly set
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	require.NotNil(t, configFlag)
	assert.Equal(t, "c", configFlag.Shorthand)
	assert.Equal(t, "", configFlag.DefValue)
	assert.Equal(t, "config file path", configFlag.Usage)

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	require.NotNil(t, verboseFlag)
	assert.Equal(t, "v", verboseFlag.Shorthand)
	assert.Equal(t, "false", verboseFlag.DefValue)
	assert.Equal(t, "verbose output", verboseFlag.Usage)
}

func TestRootCommandMetadata(t *testing.T) {
	assert.Equal(t, "notion-md-sync", rootCmd.Use)
	assert.Equal(t, "Bridge between markdown files and Notion pages", rootCmd.Short)
	assert.True(t, strings.Contains(rootCmd.Long, "bidirectional synchronization"))
	assert.Equal(t, "1.0.0", rootCmd.Version)
}
