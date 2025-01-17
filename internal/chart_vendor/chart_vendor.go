package chart_vendor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andygrunwald/go-gerrit"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"

	"github.com/vexxhost/chart-vendor/internal/config"
	"github.com/vexxhost/chart-vendor/internal/helm"
)

func Patch(logger *slog.Logger, input, directory string) error {
	includes := []string{
		fmt.Sprintf("%s/*", path.Base(directory)),
	}
	excludes := []string{
		fmt.Sprintf("%s/Chart.yaml", path.Base(directory)),
		fmt.Sprintf("%s/values_overrides/*", path.Base(directory)),
	}

	includefiles, err := os.CreateTemp("", "includes")
	if err != nil {
		return err
	}
	defer os.Remove(includefiles.Name())
	_, err = includefiles.WriteString(strings.Join(includes, "\n"))
	if err != nil {
		return err
	}

	excludefiles, err := os.CreateTemp("", "excludes")
	if err != nil {
		return err
	}
	defer os.Remove(excludefiles.Name())
	_, err = excludefiles.WriteString(strings.Join(excludes, "\n"))
	if err != nil {
		return err
	}

	includecmd := exec.Command("filterdiff", "-p1", "-I", includefiles.Name())
	excludecmd := exec.Command("filterdiff", "-p1", "-X", excludefiles.Name())
	patchcmd := exec.Command("patch", "-p2", "-d", directory, "-E")

	stdin, err := includecmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		_, err = io.WriteString(stdin, input)
	}()

	excludepipereader, excludepipewriter := io.Pipe()
	patchpipereader, patchpipewriter := io.Pipe()

	includecmd.Stdout = excludepipewriter

	excludecmd.Stdin = excludepipereader
	excludecmd.Stdout = patchpipewriter

	var patch bytes.Buffer
	patchcmd.Stdin = patchpipereader
	patchcmd.Stdout = &patch

	err = includecmd.Start()
	if err != nil {
		return err
	}

	err = excludecmd.Start()
	if err != nil {
		return err
	}

	err = patchcmd.Start()
	if err != nil {
		return err
	}

	err = includecmd.Wait()
	if err != nil {
		return errors.Join(err, fmt.Errorf("failed to run include filterdiff"))
	}

	err = excludepipewriter.Close()
	if err != nil {
		return err
	}

	err = excludecmd.Wait()
	if err != nil {
		return errors.Join(err, fmt.Errorf("failed to run exclude filterdiff"))
	}

	err = patchpipewriter.Close()
	if err != nil {
		return err
	}

	err = patchcmd.Wait()
	if err != nil {
		logger.With("error", err).Error("failed to apply patch")
		return err
	}

	return nil
}

func PatchFromFiles(logger *slog.Logger, name string, path string, directory string) error {
    patchesPath := fmt.Sprintf("%s/patches/%s", path, name)
    if _, err := os.Stat(patchesPath); err == nil {
        patches, err := filepath.Glob(
            fmt.Sprintf("%s/*.patch", patchesPath),
        )
        if err != nil {
            return err
        }

        sort.Strings(patches)

        for _, patch := range patches {
            logger = logger.With("patch", patch)

            patchData, err := os.ReadFile(patch)
            if err != nil {
                return err
            }

            logger.Info("applying patch")
            err = Patch(logger, string(patchData), fmt.Sprintf("%s/%s", path, directory))
            if err != nil {
                return err
            }
        }
    }
    return nil
}

func FetchChart(chart config.Chart, path string) error {
	logger := slog.With("chart", chart.Name, "version", chart.Version, "repository", chart.Repository.URL)

	logger.Info("fetching chart")

	directory := chart.Name
	if chart.Directory != nil {
		directory = *chart.Directory
	}

	err := os.RemoveAll(
		fmt.Sprintf("%s/%s-%s", path, directory, chart.Version),
	)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	err = os.Rename(
		fmt.Sprintf("%s/%s", path, directory),
		fmt.Sprintf("%s/%s-%s", path, directory, chart.Version),
	)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	err = helm.FetchChart(
		// TODO: session for cache?
		chart.Repository.URL,
		chart.Name,
		chart.Version,
		path,
		directory,
	)

	if err != nil {
		nerr := os.Rename(
			fmt.Sprintf("%s/%s-%s", path, directory, chart.Version),
			fmt.Sprintf("%s/%s", path, directory),
		)
		if nerr != nil {
			return nerr
		}

		return err
	}

	err = os.RemoveAll(
		fmt.Sprintf("%s/%s-%s", path, directory, chart.Version),
	)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if len(chart.Dependencies) != 0 {
		requirementdata, err := yaml.Marshal(chart.Dependencies)
		if err != nil {
			return err
		}
		fullData := append([]byte("dependencies:\n"), requirementdata...)

		err = os.WriteFile(
			fmt.Sprintf("%s/%s/requirements.yaml", path, directory),
			fullData,
			0644,
		)
		if err != nil {
			return err
		}

		g := errgroup.Group{}

		for _, dependency := range chart.Dependencies {
			g.Go(func() error {
				err := os.RemoveAll(
					fmt.Sprintf("%s/%s/charts/%s", path, directory, dependency.Name),
				)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}

				err = helm.FetchChart(
					// TODO: session for cache?
					dependency.Repository,
					dependency.Name,
					dependency.Version,
					fmt.Sprintf("%s/%s/charts", path, directory),
					dependency.Name,
				)
				if err != nil {
					return err
				}

				err = PatchFromFiles(
					logger,
					dependency.Name,
					path,
					fmt.Sprintf("%s/charts/%s", directory, dependency.Name),
				)
				if err != nil {
					return err
				}

				return helm.UpdateRequirementsLock(
					fmt.Sprintf("%s/%s/charts/%s/requirements.lock", path, directory, dependency.Name),
					nil,
				)
			})
		}

		err = g.Wait()
		if err != nil {
			return err
		}
	}

	err = helm.UpdateRequirementsLock(
		fmt.Sprintf("%s/%s/requirements.lock", path, directory),
		chart.Dependencies,
	)
	if err != nil {
		return err
	}

	for instance, changes := range chart.Patches.Gerrit {
		url := fmt.Sprintf("https://%s", instance)
		client, err := gerrit.NewClient(context.TODO(), url, nil)
		if err != nil {
			return err
		}

		for _, changeID := range changes {
			gLogger := logger.With("instance", instance, "change", changeID)

			patch, _, err := client.Changes.GetPatch(context.TODO(), changeID, "current", nil)
			if err != nil {
				return err
			}

			gLogger.Info("applying patch")
			err = Patch(gLogger, *patch, fmt.Sprintf("%s/%s", path, directory))
			if err != nil {
				return err
			}
		}
	}

	err = PatchFromFiles(logger, chart.Name, path, directory)
	if err != nil {
		return err
	}

	return nil
}
