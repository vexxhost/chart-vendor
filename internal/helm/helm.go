package helm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/provenance"
	repo "helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"
)

var (
	settings = cli.New()
)

func FetchChart(repoURL, name, version, path, directory string) error {
	getters := getter.All(settings)

	url, err := repo.FindChartInRepoURL(
		repoURL,
		name,
		getters,
		repo.WithChartVersion(version),
	)
	if err != nil {
		return err
	}

	dl := downloader.ChartDownloader{
		Out:          os.Stderr,
		Getters:      getters,
		ContentCache: helmpath.CachePath("content"),
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
