package olmv1

import (
	"strings"

	"github.com/spf13/cobra"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/operator-framework/kubectl-operator/internal/cmd/internal/log"
	experimentalaction "github.com/operator-framework/kubectl-operator/internal/pkg/experimental/action"
	"github.com/operator-framework/kubectl-operator/pkg/action"
)

func NewPackageCmd(cfg *action.Configuration) *cobra.Command {
	restcfg := ctrl.GetConfigOrDie()

	var cacheDir string
	var clusterCatalogs string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Gets a set of packages available for install",
		Run: func(cmd *cobra.Command, args []string) {
			i := experimentalaction.NewPackageLister(cfg, restcfg, cacheDir, catalogList(clusterCatalogs))
			i.Logf = log.Printf

			err := i.Run(cmd.Context())
			if err != nil {
				log.Fatalf("failed to get packages: %v", err)
			}
		},
	}

	cmd.Flags().StringVarP(&cacheDir, "cache-path", "d", "", "cache directory for local package data")
	cmd.Flags().StringVarP(&clusterCatalogs, "cluster-catalogs", "c", "", "list of cluster catalog to filter packages from")

	return cmd
}

func catalogList(clusterCatalogs string) []string {
	var catalogList []string
	rawCatalogList := strings.Split(clusterCatalogs, ",")
	for _, catalog := range rawCatalogList {
		catalogList = append(catalogList, strings.ToLower(catalog))
	}
	return catalogList
}
