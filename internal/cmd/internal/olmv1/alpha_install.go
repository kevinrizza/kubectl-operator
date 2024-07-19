package olmv1

import (
	"github.com/spf13/cobra"

	"github.com/operator-framework/kubectl-operator/internal/cmd/internal/log"
	experimentalaction "github.com/operator-framework/kubectl-operator/internal/pkg/experimental/action"
	"github.com/operator-framework/kubectl-operator/pkg/action"
)

func NewOperatorInstallCmd(cfg *action.Configuration) *cobra.Command {
	i := experimentalaction.NewOperatorInstall(cfg)
	i.Logf = log.Printf
	var version string
	var installNamespace string
	var serviceAccount string

	cmd := &cobra.Command{
		Use:   "install <clusterextension>",
		Short: "Install a cluster extension",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			i.Package = args[0]
			i.Version = version
			i.InstallNamespace = installNamespace
			i.ServiceAccount = serviceAccount
			_, err := i.Run(cmd.Context())
			if err != nil {
				log.Fatalf("failed to install operator: %v", err)
			}
			log.Printf("cluster extension %q created", i.Package)
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "", "version of package to install")
	cmd.Flags().StringVarP(&installNamespace, "install-namespace", "i", "default", "namespace to use to install")
	cmd.Flags().StringVarP(&serviceAccount, "service-account", "s", "default", "service account to use to install")

	return cmd
}
