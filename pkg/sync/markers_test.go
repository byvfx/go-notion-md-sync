package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEditedKeptRegions(t *testing.T) {
	const id = "225388d7-7461-80f3-b15c-cf7cc57c3f3b"
	region := func(body string) string {
		return "intro\n\n" + regionBegin("table", id) + "\n\n" + body + "\n\n" + regionEnd(id) + "\n\noutro"
	}
	table := "| a | b |\n| --- | --- |\n| 1 | 2 |"

	tests := []struct {
		name          string
		local, remote string
		want          []string
	}{
		{"unchanged", region(table), region(table), nil},
		{"only text around it changed", "changed intro" + region(table)[5:], region(table), nil},
		{"blank lines and indentation differ", region("  " + table + "\n\n\n"), region(table), nil},
		{"cell edited", region("| a | b |\n| --- | --- |\n| 1 | EDIT |"), region(table), []string{normalizeID(id)}},
		{"region removed locally", "intro\n\noutro", region(table), nil},
		{"unclosed local region", "intro\n\n" + regionBegin("table", id) + "\n\nedited", region(table), nil},
		{"region quoted in code", "```\n" + region("edited") + "\n```", region(table), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, editedKeptRegions(tt.local, tt.remote))
		})
	}
}
