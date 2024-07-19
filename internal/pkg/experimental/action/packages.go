package action

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"

	"github.com/operator-framework/kubectl-operator/pkg/action"

	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/cache"
	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/client"
	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/portforward"
)

type PackageLister struct {
	config      *action.Configuration
	restcfg     *rest.Config
	cacheDir    string
	catalogList []string

	Logf func(string, ...interface{})
}

func NewPackageLister(cfg *action.Configuration, restcfg *rest.Config, cacheDir string, catalogList []string) *PackageLister {
	return &PackageLister{
		config:      cfg,
		restcfg:     restcfg,
		cacheDir:    cacheDir,
		catalogList: catalogList,
		Logf:        func(string, ...interface{}) {},
	}
}

func (l *PackageLister) Run(ctx context.Context) error {
	catalogdCA, err := portforward.GetClusterCA(ctx, l.config.Client)
	if err != nil {
		return fmt.Errorf("unable to get catalog CA: %w", err)
	}

	cacheDir := ""
	if l.cacheDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			//couldn't detect a home directory, so using local dir
			cacheDir = "./operator-framework/cache"
		}
		cacheDir = filepath.Join(homeDir, "/operator-framework/cache")
	}

	fsCache, err := cache.NewFilesystemCache(cacheDir, l.restcfg, catalogdCA)
	if err != nil {
		return fmt.Errorf("unable to create filesystem cache: %w", err)
	}

	catalogClient := client.New(l.config.Client, fsCache)

	// get list of packages
	allPackages, err := catalogClient.GetPackages(ctx, l.catalogList)
	if err != nil {
		return fmt.Errorf("error fetching bundles: %w", err)
	}
	for _, pkg := range allPackages {
		fmt.Println(pkg)
	}

	return nil
}
