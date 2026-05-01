package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xueyulinn/cicd-system/internal/common/gitutil"
)

func repoFromCommandContext(cmd *cobra.Command) (*gitutil.Repository, error) {
	if cmd == nil {
		return nil, fmt.Errorf("git repository context is missing")
	}

	ctx := cmd.Context()
	if ctx == nil {
		return nil, fmt.Errorf("git repository context is missing")
	}

	repo, ok := ctx.Value(repoKey).(*gitutil.Repository)
	if !ok || repo == nil {
		return nil, fmt.Errorf("git repository context is missing")
	}

	return repo, nil
}
