package client

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
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

func (c *Client) Packages(ctx context.Context) ([]string, error) {
	var allPackages []string

	var catalogList catalogd.ClusterCatalogList
	if err := c.cl.List(ctx, &catalogList); err != nil {
		return nil, err
	}

	var errs []error
	for _, catalog := range catalogList.Items {
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

func PopulateExtraFields(catalogName string, channels []*catalogmetadata.Channel, bundles []*catalogmetadata.Bundle, deprecations []*catalogmetadata.Deprecation) ([]*catalogmetadata.Bundle, error) {
	bundlesMap := map[string]*catalogmetadata.Bundle{}
	for i := range bundles {
		bundleKey := fmt.Sprintf("%s-%s", bundles[i].Package, bundles[i].Name)
		bundlesMap[bundleKey] = bundles[i]

		bundles[i].CatalogName = catalogName
	}

	for _, ch := range channels {
		for _, chEntry := range ch.Entries {
			bundleKey := fmt.Sprintf("%s-%s", ch.Package, chEntry.Name)
			bundle, ok := bundlesMap[bundleKey]
			if !ok {
				return nil, fmt.Errorf("bundle %q not found in catalog %q (package %q, channel %q)", chEntry.Name, catalogName, ch.Package, ch.Name)
			}

			bundle.InChannels = append(bundle.InChannels, ch)
		}
	}

	// We sort the channels here because the order that channels appear in this list is non-deterministic.
	// They are non-deterministic because they are originally read from the cache in a concurrent manner that
	// provides no ordering guarantees.
	//
	// This sort isn't strictly necessary for correctness, but it makes the output consistent and easier to
	// reason about.
	for _, bundle := range bundles {
		slices.SortFunc(bundle.InChannels, func(a, b *catalogmetadata.Channel) int { return cmp.Compare(a.Name, b.Name) })
	}

	// According to https://docs.google.com/document/d/1EzefSzoGZL2ipBt-eCQwqqNwlpOIt7wuwjG6_8ZCi5s/edit?usp=sharing
	// the olm.deprecations FBC object is only valid when either 0 or 1 instances exist
	// for any given package
	deprecationMap := make(map[string]*catalogmetadata.Deprecation, len(deprecations))
	for _, deprecation := range deprecations {
		deprecationMap[deprecation.Package] = deprecation
	}

	for i := range bundles {
		if dep, ok := deprecationMap[bundles[i].Package]; ok {
			for _, entry := range dep.Entries {
				switch entry.Reference.Schema {
				case declcfg.SchemaPackage:
					bundles[i].Deprecations = append(bundles[i].Deprecations, entry)
				case declcfg.SchemaChannel:
					for _, ch := range bundles[i].InChannels {
						if ch.Name == entry.Reference.Name {
							bundles[i].Deprecations = append(bundles[i].Deprecations, entry)
							break
						}
					}
				case declcfg.SchemaBundle:
					if bundles[i].Name == entry.Reference.Name {
						bundles[i].Deprecations = append(bundles[i].Deprecations, entry)
					}
				}
			}
		}
	}

	return bundles, nil
}
