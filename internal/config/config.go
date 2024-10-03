package config

import (
	"os"
	"sync"

	"helm.sh/helm/v3/pkg/chart"
	"sigs.k8s.io/yaml"
)

type ChartRepository struct {
	URL string `yaml:"url"`
}

type ChartPatches struct {
	Gerrit map[string][]string `yaml:"gerrit"`
}

type Chart struct {
	Name         string              `yaml:"name"`
	Version      string              `yaml:"version"`
	Repository   ChartRepository     `yaml:"repository"`
	Directory    *string             `yaml:"directory"`
	Dependencies []*chart.Dependency `yaml:"dependencies"`
	Patches      ChartPatches        `yaml:"patches"`
}

type Config struct {
	Charts []Chart `yaml:"charts"`
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
