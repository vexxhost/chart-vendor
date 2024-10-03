package main

import (
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/go-git/go-git/v5"
	"github.com/urfave/cli/v2"
	"github.com/vexxhost/chart-vendor/internal/chart_vendor"
	"github.com/vexxhost/chart-vendor/internal/config"
	"golang.org/x/sync/errgroup"
)

func main() {
	app := &cli.App{
		Name: "Chart Vendor CLI",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config-file",
				Usage: "Configuration file for the vendored charts",
				Value: ".charts.yml",
			},
			&cli.StringFlag{
				Name:  "charts-root",
				Usage: "Root path where charts are generated",
				Value: "charts",
			},
			&cli.BoolFlag{
				Name:  "check",
				Usage: "Check if all chart manifests are applied or not",
				Value: false,
			},
		},
		Action: func(ctx *cli.Context) error {
			configFile := ctx.String("config-file")

			parsedConfig, err := config.ParseFromFile(configFile)
			if err != nil {
				return err
			}

			g := errgroup.Group{}
			selectedCharts := ctx.Args().Slice()

			for _, chart := range parsedConfig.Charts {
				if len(selectedCharts) != 0 && !slices.Contains(selectedCharts, chart.Name) {
					continue
				}

				g.Go(func() error {
					return chart_vendor.FetchChart(chart, ctx.String("charts-root"))
				})
			}

			err = g.Wait()
			if err != nil {
				return err
			}

			if ctx.Bool("check") {
				repo, err := git.PlainOpen(".")
				if err != nil {
					return err
				}

				worktree, err := repo.Worktree()
				if err != nil {
					return err
				}

				status, err := worktree.Status()
				if err != nil {
					return err
				}

				passed := true

				for file, stat := range status {
					if stat.Staging != git.Unmodified || stat.Worktree != git.Unmodified {
						log.Printf("Changed file: %s\n", file)
						passed = false
					}

					if stat.Worktree == git.Untracked {
						log.Printf("Untracked file: %s\n", file)
						passed = false
					}
				}

				if !passed {
					return fmt.Errorf("uncommitted changes or untracked files found")
				} else {
					log.Println("No uncommitted changes or untracked files.")
				}
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
