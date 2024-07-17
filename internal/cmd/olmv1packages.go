package cmd

import (
	"github.com/spf13/cobra"

	"github.com/operator-framework/kubectl-operator/internal/cmd/internal/olmv1"
	"github.com/operator-framework/kubectl-operator/pkg/action"
)

func newOlmV1PackagesCmd(cfg *action.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "olmv1packages",
		Short: "Manage packages via OLMv1 in a cluster from the command line",
		Long:  "Manage packages via OLMv1 in a cluster from the command line.",
	}

	cmd.AddCommand(
		olmv1.NewPackageCmd(cfg),
	)

	return cmd
}
