package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelpReturnsSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--help"}, &stdout, &stderr); status != 0 {
		t.Fatalf("help returned status %d", status)
	}
	if !strings.Contains(stderr.String(), `Output directory (default "./exported-wiki")`) {
		t.Fatalf("help does not show the output directory default: %s", stderr.String())
	}
}

func TestSelectWiki(t *testing.T) {
	enabled := Wiki{ID: "enabled-id", Name: "Enabled"}
	disabled := Wiki{ID: "disabled-id", Name: "Disabled", IsDisabled: true}
	project := "project"

	tests := []struct {
		name      string
		wikis     []Wiki
		selector  string
		wantID    string
		wantError string
	}{
		{
			name:      "no wikis",
			wantError: `no enabled wikis found in project "project"`,
		},
		{
			name:      "only disabled wikis",
			wikis:     []Wiki{disabled},
			wantError: `no enabled wikis found in project "project"`,
		},
		{
			name:   "one enabled wiki",
			wikis:  []Wiki{disabled, enabled},
			wantID: enabled.ID,
		},
		{
			name:      "multiple enabled wikis",
			wikis:     []Wiki{enabled, {ID: "other-id", Name: "Other"}},
			wantError: "specify --wiki; the project has multiple enabled wikis",
		},
		{
			name:      "explicitly selected disabled wiki",
			wikis:     []Wiki{disabled, enabled},
			selector:  disabled.ID,
			wantError: "wiki is disabled: Disabled",
		},
		{
			name:      "unknown selection",
			wikis:     []Wiki{enabled},
			selector:  "missing",
			wantError: `no wiki found matching --wiki "missing"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wiki, err := selectWiki(test.wikis, test.selector, project)
			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("selectWiki() error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectWiki() error = %v", err)
			}
			if wiki.ID != test.wantID {
				t.Fatalf("selectWiki() wiki ID = %q, want %q", wiki.ID, test.wantID)
			}
		})
	}
}
