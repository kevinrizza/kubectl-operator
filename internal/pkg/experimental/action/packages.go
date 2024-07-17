package action

import (
	"context"
	"fmt"

	//"k8s.io/apimachinery/pkg/api/meta"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	//olmv1 "github.com/operator-framework/operator-controller/api/v1alpha1"

	"github.com/operator-framework/kubectl-operator/pkg/action"

	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/cache"
	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/client"
	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/httputil"
	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/portforward"
	//"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/util"
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
	//httpClient, err := httputil.BuildHTTPClient(caCertDir)
	httpClient, err := httputil.BuildHTTPClient("./certs")
	if err != nil {
		return fmt.Errorf("unable to create http client to connect to catalog service: %w", err)
	}

	catalogdCA, err := portforward.GetClusterCA(ctx, l.config.Client, types.NamespacedName{Namespace: "olmv1-system", Name: "catalogd-catalogserver-cert"})
	if err != nil {
		return err
	}

	fsCache, err := cache.NewFilesystemCache("./localcache", l.restcfg, httpClient, catalogdCA)
	if err != nil {
		return fmt.Errorf("unable to create filesystem cache: %w", err)
	}

	catalogClient := client.New(l.config.Client, fsCache)

	// query for catalog connection details

	// configure port forward if the connection details are there

	// get list of packages
	allPackages, err := catalogClient.Packages(ctx)
	if err != nil {
		return fmt.Errorf("error fetching bundles: %w", err)
	}
	for _, pkg := range allPackages {
		fmt.Println(pkg)
	}

	return nil
}
