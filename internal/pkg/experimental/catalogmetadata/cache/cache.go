package cache

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"

	catalogd "github.com/operator-framework/catalogd/api/core/v1alpha1"
	"github.com/operator-framework/operator-registry/alpha/declcfg"

	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/client"
	"github.com/operator-framework/kubectl-operator/internal/pkg/experimental/catalogmetadata/portforward"
)

var _ client.Fetcher = &filesystemCache{}

// NewFilesystemCache returns a client.Fetcher implementation that uses a
// local filesystem to cache Catalog contents. When fetching the Catalog contents
// it will:
// - Check if the Catalog is cached on disk
//   - IF !cached it will fetch from the catalogd HTTP server and cache the response
//   - IF cached it will verify the cache is up to date. If it is up to date it will return
//     the cached contents, if not it will fetch the new contents from the catalogd HTTP
//     server and update the cached contents.
func NewFilesystemCache(cachePath string, restcfg *rest.Config, rootCAs *x509.CertPool) (client.Fetcher, error) {
	cacheDataMap := map[string]cacheData{}
	cacheFile := filepath.Join(cachePath, "cache.json")

	cacheJson, err := os.ReadFile(cacheFile)
	if err == nil {
		if err := json.Unmarshal(cacheJson, &cacheDataMap); err != nil {
			return nil, fmt.Errorf("unable to parse local cache data %s: %w", cacheFile, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return &filesystemCache{
		cachePath:              cachePath,
		cacheFile:              cacheFile,
		restcfg:                restcfg,
		rootCAs:                rootCAs,
		cacheDataByCatalogName: cacheDataMap,
	}, nil
}

// cacheData holds information about a catalog
// other than it's contents that is used for
// making decisions on when to attempt to refresh
// the cache.
type cacheData struct {
	ResolvedRef string
}

// FilesystemCache is a cache that
// uses the local filesystem for caching
// catalog contents. It will fetch catalog
// contents if the catalog does not already
// exist in the cache.
type filesystemCache struct {
	cachePath              string
	cacheFile              string
	restcfg                *rest.Config
	rootCAs                *x509.CertPool
	cacheDataByCatalogName map[string]cacheData
}

// FetchCatalogContents implements the client.Fetcher interface and
// will fetch the contents for the provided Catalog from the filesystem.
// If the provided Catalog has not yet been cached, it will make a GET
// request to the Catalogd HTTP server to get the Catalog contents and cache
// them. The cache will be updated automatically if a Catalog is noticed to
// have a different resolved image reference.
// The Catalog provided to this function is expected to:
// - Be non-nil
// - Have a non-nil Catalog.Status.ResolvedSource.Image
// This ensures that we are only attempting to fetch catalog contents for Catalog
// resources that have been successfully reconciled, unpacked, and are being served.
// These requirements help ensure that we can rely on status conditions to determine
// when to issue a request to update the cached Catalog contents.
func (fsc *filesystemCache) FetchCatalogContents(ctx context.Context, catalog *catalogd.ClusterCatalog) (fs.FS, error) {
	if catalog == nil {
		return nil, fmt.Errorf("error: provided catalog must be non-nil")
	}

	if catalog.Status.ResolvedSource == nil {
		return nil, fmt.Errorf("error: catalog %q has a nil status.resolvedSource value", catalog.Name)
	}

	if catalog.Status.ResolvedSource.Image == nil {
		return nil, fmt.Errorf("error: catalog %q has a nil status.resolvedSource.image value", catalog.Name)
	}

	// create cache dir if it doesn't yet exist
	err := os.MkdirAll(fsc.cachePath, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("error: unable to create cache directory: %w", err)
	}

	cacheDir := filepath.Join(fsc.cachePath, catalog.Name)
	if data, ok := fsc.cacheDataByCatalogName[catalog.Name]; ok {
		if catalog.Status.ResolvedSource.Image.ResolvedRef == data.ResolvedRef {
			return os.DirFS(cacheDir), nil
		}
	}

	// configure port forwarder to query catalog service on cluster
	pf, err := portforward.NewServicePortForwarder(fsc.restcfg, types.NamespacedName{Namespace: "olmv1-system", Name: "catalogd-catalogserver"}, intstr.FromString("https"))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return pf.Start(ctx)
	})

	eg.Go(func() error {
		defer cancel()

		localPort, err := pf.LocalPort(ctx)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyCatalogURL(catalog.Name, localPort), nil)
		if err != nil {
			return err
		}

		if fsc.restcfg.BearerToken != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", fsc.restcfg.BearerToken))
		}

		httpClient, err := portforward.NewHttpClient(fsc.restcfg, fsc.rootCAs)
		if err != nil {
			return fmt.Errorf("error creating http client to get catalog data: %w", err)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("error performing request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error: received unexpected response status code %d", resp.StatusCode)
		}

		tmpDir, err := os.MkdirTemp(fsc.cachePath, fmt.Sprintf(".%s-", catalog.Name))
		if err != nil {
			return fmt.Errorf("error creating temporary directory to unpack catalog metadata: %v", err)
		}

		if err := declcfg.WalkMetasReader(resp.Body, func(meta *declcfg.Meta, err error) error {
			if err != nil {
				return fmt.Errorf("error parsing catalog contents: %v", err)
			}
			pkgName := meta.Package
			if meta.Schema == declcfg.SchemaPackage {
				pkgName = meta.Name
			}
			metaName := meta.Name
			if meta.Name == "" {
				metaName = meta.Schema
			}
			metaPath := filepath.Join(tmpDir, pkgName, meta.Schema, metaName+".json")
			if err := os.MkdirAll(filepath.Dir(metaPath), os.ModePerm); err != nil {
				return fmt.Errorf("error creating directory for catalog metadata: %v", err)
			}
			if err := os.WriteFile(metaPath, meta.Blob, os.ModePerm); err != nil {
				return fmt.Errorf("error writing catalog metadata to file: %v", err)
			}
			return nil
		}); err != nil {
			return err
		}

		if err := os.RemoveAll(cacheDir); err != nil {
			return fmt.Errorf("error removing old cache directory: %v", err)
		}
		if err := os.Rename(tmpDir, cacheDir); err != nil {
			return fmt.Errorf("error moving temporary directory to cache directory: %v", err)
		}

		fsc.cacheDataByCatalogName[catalog.Name] = cacheData{
			ResolvedRef: catalog.Status.ResolvedSource.Image.ResolvedRef,
		}
		return nil
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}

	cacheDataToWrite, err := json.Marshal(fsc.cacheDataByCatalogName)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal in memory cache data: %w", err)
	}
	err = os.WriteFile(fsc.cacheFile, cacheDataToWrite, 0644)
	if err != nil {
		return nil, fmt.Errorf("unable to write cache file: %w", err)
	}

	return os.DirFS(cacheDir), nil
}

func proxyCatalogURL(catalog string, port uint16) string {
	return fmt.Sprintf("https://localhost:%d/catalogs/%s/all.json", port, catalog)
}
