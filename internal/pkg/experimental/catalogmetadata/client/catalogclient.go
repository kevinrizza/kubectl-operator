package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	catalogd "github.com/operator-framework/catalogd/api/core/v1alpha1"
	"github.com/operator-framework/operator-registry/alpha/declcfg"

	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata"
)

// Fetcher is an interface to facilitate fetching
// catalog contents from catalogd.
type Fetcher interface {
	// FetchCatalogContents fetches contents from the catalogd HTTP
	// server for the catalog provided. It returns an io.ReadCloser
	// containing the FBC contents that the caller is expected to close.
	// returns an error if any occur.
	FetchCatalogContents(ctx context.Context, catalog *catalogd.ClusterCatalog) (fs.FS, error)
}

func New(cl client.Client, fetcher Fetcher) *Client {
	return &Client{
		cl:      cl,
		fetcher: fetcher,
	}
}

// Client is reading catalog metadata
type Client struct {
	// Note that eventually we will be reading from catalogd http API
	// instead of kube API server. We will need to swap this implementation.
	cl client.Client

	// rest config to generate client-go rest client
	cfg *rest.Config

	// fetcher is the Fetcher to use for fetching catalog contents
	fetcher Fetcher
}

func (c *Client) GetPackages(ctx context.Context, catalogs []string) ([]string, error) {
	var allPackages []string

	var catalogList catalogd.ClusterCatalogList
	if err := c.cl.List(ctx, &catalogList); err != nil {
		return nil, err
	}

	var errs []error
	for _, catalog := range catalogList.Items {
		if len(catalogs) > 0 && !slices.Contains(catalogs, strings.ToLower(catalog.Name)) {
			// filter that catalog from the result
			continue
		}
		// if the catalog has not been successfully unpacked, report an error. This ensures that our
		// reconciles are deterministic and wait for all desired catalogs to be ready.
		if !meta.IsStatusConditionPresentAndEqual(catalog.Status.Conditions, catalogd.TypeUnpacked, metav1.ConditionTrue) {
			errs = append(errs, fmt.Errorf("catalog %q is not unpacked", catalog.Name))
			continue
		}

		var packagesMu sync.Mutex

		catalogFS, err := c.fetcher.FetchCatalogContents(ctx, catalog.DeepCopy())
		if err != nil {
			errs = append(errs, fmt.Errorf("error fetching catalog %q contents: %v", catalog.Name, err))
			continue
		}

		if err := declcfg.WalkMetasFS(ctx, catalogFS, func(_ string, meta *declcfg.Meta, err error) error {
			if err != nil {
				return fmt.Errorf("error parsing package metadata: %v", err)
			}

			switch meta.Schema {
			case declcfg.SchemaPackage:
				var content catalogmetadata.Package
				if err := json.Unmarshal(meta.Blob, &content); err != nil {
					return fmt.Errorf("error unmarshalling package from catalog metadata: %v", err)
				}
				packagesMu.Lock()
				defer packagesMu.Unlock()
				pkgName := content.Name
				allPackages = append(allPackages, pkgName)
			}
			return nil
		}); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("error reading packages from catalog %v", catalog.Name, err))
			}
			continue
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return allPackages, nil
}
