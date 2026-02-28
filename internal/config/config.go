package config

import (
	"os"
	"sync"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"sigs.k8s.io/yaml"
)

// ChartRepository holds the URL of a Helm chart repository.
type ChartRepository struct {
	URL string `json:"url" yaml:"url" jsonschema:"description=URL of the Helm chart repository,format=uri"`
}

// ChartPatches holds the patches to apply to a vendored chart.
type ChartPatches struct {
	Gerrit map[string][]string `json:"gerrit,omitempty" yaml:"gerrit" jsonschema:"description=Gerrit patch sets to apply keyed by Gerrit host"`
}

// Chart holds the configuration for a single vendored Helm chart.
type Chart struct {
	Name         string              `json:"name" yaml:"name" jsonschema:"description=Name of the chart"`
	Version      string              `json:"version" yaml:"version" jsonschema:"description=Version of the chart"`
	Repository   ChartRepository     `json:"repository" yaml:"repository" jsonschema:"description=Helm chart repository configuration"`
	Directory    *string             `json:"directory,omitempty" yaml:"directory" jsonschema:"description=Optional directory name override for the vendored chart"`
	Dependencies []*chart.Dependency `json:"dependencies,omitempty" yaml:"dependencies" jsonschema:"description=List of chart dependencies"`
	Patches      ChartPatches        `json:"patches,omitempty" yaml:"patches" jsonschema:"description=Patches to apply to the vendored chart"`
}

// Config is the top-level structure of the .charts.yml configuration file.
type Config struct {
	Charts []Chart `json:"charts" yaml:"charts" jsonschema:"description=List of Helm charts to vendor"`
}

func ParseFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) FetchChart(chart Chart, path string, wg *sync.WaitGroup) error {
	return nil
}
