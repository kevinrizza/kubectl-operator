package action

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	"github.com/operator-framework/kubectl-operator/pkg/action"

	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/cache"
	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/client"
	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/portforward"
)

type PackageLister struct {
	config  *action.Configuration
	restcfg *rest.Config

	Logf func(string, ...interface{})
}

func NewPackageLister(cfg *action.Configuration, restcfg *rest.Config) *PackageLister {
	return &PackageLister{
		config:  cfg,
		restcfg: restcfg,
		Logf:    func(string, ...interface{}) {},
	}
}

func (l *PackageLister) Run(ctx context.Context) error {
	catalogdCA, err := portforward.GetClusterCA(ctx, l.config.Client, types.NamespacedName{Namespace: "olmv1-system", Name: "catalogd-catalogserver-cert"})
	if err != nil {
		return fmt.Errorf("unable to get catalog CA: %w", err)
	}

	fsCache, err := cache.NewFilesystemCache("./localcache", l.restcfg, catalogdCA)
	if err != nil {
		return fmt.Errorf("unable to create filesystem cache: %w", err)
	}

	catalogClient := client.New(l.config.Client, fsCache)

	// get list of packages
	allPackages, err := catalogClient.GetPackages(ctx)
	if err != nil {
		return fmt.Errorf("error fetching bundles: %w", err)
	}
	for _, pkg := range allPackages {
		fmt.Println(pkg)
	}

	return nil
}
