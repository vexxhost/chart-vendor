package helm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/provenance"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
)

var (
	settings = &cli.EnvSettings{
		// RepositoryConfig: repoConfig,
		// RepositoryCache:  repoCache,
	}
)

func FetchChart(repoURL, name, version, path, directory string) error {
	getters := getter.All(settings)

	url, err := repo.FindChartInRepoURL(
		repoURL,
		name,
		version,
		"", "", "", getters,
	)
	if err != nil {
		return err
	}

	dl := downloader.ChartDownloader{
		Out: os.Stderr,
		// RepositoryConfig: repoConfig,
		// RepositoryCache:  repoCache,
		Getters: getters,
	}

	chartPath, _, err := dl.DownloadTo(url, version, path)
	if err != nil {
		return err
	}

	err = chartutil.ExpandFile(path, chartPath)
	if err != nil {
		return err
	}

	err = os.Remove(chartPath)
	if err != nil {
		return err
	}

	if name != directory {
		err = os.Rename(
			fmt.Sprintf("%s/%s", path, name),
			fmt.Sprintf("%s/%s", path, directory),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func UpdateRequirementsLock(path string, req []*chart.Dependency) error {
	if req == nil {
		req = []*chart.Dependency{}
	}

	data, err := json.Marshal([2][]*chart.Dependency{req, req})
	if err != nil {
		return err
	}

	digest, err := provenance.Digest(bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	lock := &chart.Lock{
		Generated:    time.Time{},
		Dependencies: req,
		Digest:       fmt.Sprintf("sha256:%s", digest),
	}

	ldata, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}

	return os.WriteFile(path, ldata, 0644)
}
