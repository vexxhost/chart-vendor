package chart_vendor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// diffSectionFilename returns the relevant filename for a diff section by
// examining the --- and +++ lines. Falls back to the diff --git header.
func diffSectionFilename(section []string) string {
	var fromPath, toPath string
	for _, line := range section {
		if strings.HasPrefix(line, "--- ") {
			parts := strings.Fields(strings.TrimPrefix(line, "--- "))
			if len(parts) > 0 && parts[0] != "/dev/null" {
				fromPath = parts[0]
			}
		} else if strings.HasPrefix(line, "+++ ") {
			parts := strings.Fields(strings.TrimPrefix(line, "+++ "))
			if len(parts) > 0 && parts[0] != "/dev/null" {
				toPath = parts[0]
			}
		} else if strings.HasPrefix(line, "@@ ") {
			break
		}
	}
	if fromPath != "" {
		return fromPath
	}
	if toPath != "" {
		return toPath
	}
	// Fallback: parse from "diff --git a/X b/Y" header.
	if len(section) > 0 && strings.HasPrefix(section[0], "diff --git ") {
		rest := strings.TrimPrefix(section[0], "diff --git ")
		if idx := strings.LastIndex(rest, " b/"); idx >= 0 && idx+3 <= len(rest) {
			aPath := rest[:idx]
			bPath := "b/" + rest[idx+3:]
			if aPath == "a/dev/null" {
				return bPath
			}
			return aPath
		}
	}
	return ""
}

// stripPathComponents removes n leading path components from p.
// e.g. stripPathComponents("a/chart/file.yaml", 1) returns "chart/file.yaml".
func stripPathComponents(p string, n int) string {
	for i := 0; i < n; i++ {
		idx := strings.Index(p, "/")
		if idx < 0 {
			return p
		}
		p = p[idx+1:]
	}
	return p
}

// matchWildcard reports whether name matches pattern, where '*' matches any
// sequence of characters (including '/') and '?' matches any single character.
func matchWildcard(pattern, name string) bool {
	pi, ni := 0, 0
	starIdx := -1
	starMatch := 0
	for ni < len(name) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == name[ni]) {
			pi++
			ni++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			starIdx = pi
			starMatch = ni
			pi++
		} else if starIdx >= 0 {
			pi = starIdx + 1
			starMatch++
			ni = starMatch
		} else {
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

// filterDiff returns a filtered copy of the unified diff in input, keeping
// only the sections for files whose path (after stripping stripComponents
// leading path components) matches at least one include pattern and no
// exclude pattern. Preamble lines before the first diff header are preserved.
func filterDiff(input string, stripComponents int, includes, excludes []string) string {
	lines := strings.Split(input, "\n")
	var result []string

	i := 0
	// Preserve preamble lines before the first "diff " header.
	for i < len(lines) && !strings.HasPrefix(lines[i], "diff ") {
		result = append(result, lines[i])
		i++
	}

	// Process each per-file section.
	for i < len(lines) {
		sectionStart := i
		i++
		for i < len(lines) && !strings.HasPrefix(lines[i], "diff ") {
			i++
		}
		section := lines[sectionStart:i]

		filename := diffSectionFilename(section)
		strippedFilename := stripPathComponents(filename, stripComponents)

		excluded := false
		for _, pat := range excludes {
			if matchWildcard(pat, strippedFilename) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		for _, pat := range includes {
			if matchWildcard(pat, strippedFilename) {
				result = append(result, section...)
				break
			}
		}
	}

	return strings.Join(result, "\n")
}

func Patch(logger *slog.Logger, input, directory string) error {
	includes := []string{
		fmt.Sprintf("%s/*", path.Base(directory)),
	}
	excludes := []string{
		fmt.Sprintf("%s/Chart.yaml", path.Base(directory)),
		fmt.Sprintf("%s/values_overrides/*", path.Base(directory)),
	}

	filtered := filterDiff(input, 1, includes, excludes)

	var patchOutput bytes.Buffer
	patchcmd := exec.Command("patch", "-p2", "-d", directory, "-E")
	patchcmd.Stdin = strings.NewReader(filtered)
	patchcmd.Stdout = &patchOutput

	err := patchcmd.Run()
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
