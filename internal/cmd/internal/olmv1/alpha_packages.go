package olmv1

import (
	"github.com/spf13/cobra"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/operator-framework/kubectl-operator/internal/cmd/internal/log"
	experimentalaction "github.com/operator-framework/kubectl-operator/internal/pkg/experimental/action"
	"github.com/operator-framework/kubectl-operator/pkg/action"
)

func NewPackageCmd(cfg *action.Configuration) *cobra.Command {
	restcfg := ctrl.GetConfigOrDie()
	i := experimentalaction.NewPackageLister(cfg, restcfg)
	i.Logf = log.Printf

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Gets a set of packages available for install",
		Run: func(cmd *cobra.Command, args []string) {
			err := i.Run(cmd.Context())
			if err != nil {
				log.Fatalf("failed to get packages: %v", err)
			}
		},
	}

	return cmd
}
