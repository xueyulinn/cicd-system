package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestFindTypedSubCmd(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		wantCmd string
	}{
		{
			name:    "unknown command extracts typed",
			errMsg:  `unknown command "dryruu" for "cicd"`,
			wantCmd: "dryruu",
		},
		{
			name:    "missing prefix returns empty",
			errMsg:  "some other error",
			wantCmd: "",
		},
		{
			name:    "missing quotes returns empty",
			errMsg:  "unknown command dryruu for cicd",
			wantCmd: "",
		},
	}

	for _, tc := range tests {
		got := findTypedSubCmd(tc.errMsg)
		if got != tc.wantCmd {
			t.Errorf("%s: expected %q, got %q", tc.name, tc.wantCmd, got)
		}
	}
}

func TestFindCommandByName(t *testing.T) {
	root := &cobra.Command{Use: "cicd"}
	child := &cobra.Command{Use: "verify", Aliases: []string{"v"}}
	nested := &cobra.Command{Use: "group"}
	grandChild := &cobra.Command{Use: "dryrun"}
	nested.AddCommand(grandChild)
	root.AddCommand(child, nested)

	if got := findCommandByName(root, "verify"); got == nil || got.Name() != "verify" {
		t.Fatalf("expected to find verify command, got: %#v", got)
	}
	if got := findCommandByName(root, "v"); got == nil || got.Name() != "verify" {
		t.Fatalf("expected to find verify via alias, got: %#v", got)
	}
	if got := findCommandByName(root, "dryrun"); got == nil || got.Name() != "dryrun" {
		t.Fatalf("expected to find nested dryrun command, got: %#v", got)
	}
	if got := findCommandByName(root, "missing"); got != nil {
		t.Fatalf("expected nil for missing command, got: %#v", got)
	}
}

