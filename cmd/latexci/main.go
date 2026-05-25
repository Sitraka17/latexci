package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sitrakaforler/latexci/internal/compiler"
	"github.com/sitrakaforler/latexci/internal/config"
	"github.com/sitrakaforler/latexci/internal/report"
	"github.com/sitrakaforler/latexci/internal/watcher"
)

var (
	cfgFile      string
	reportFlag   bool
	verboseFlag  bool
)

func main() {
	root := &cobra.Command{
		Use:   "latexci",
		Short: "Minimalist CI/CD pipeline for LaTeX projects",
		Long:  "latexci compiles LaTeX projects with structured error reporting and GitHub Actions support.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelInfo
			if verboseFlag {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
		},
	}

	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", config.DefaultConfigFile, "config file")
	root.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "verbose output")

	root.AddCommand(
		buildCmd(),
		watchCmd(),
		cleanCmd(),
		initCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// ---------- build ----------

func buildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compile the LaTeX project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			rep, exitCode := runBuild(cfg)
			rep.Print()

			if reportFlag {
				if err := rep.WriteJSON("latexci-report.json"); err != nil {
					slog.Warn("could not write report", "err", err)
				} else {
					fmt.Println("report written to latexci-report.json")
				}
			}

			// Write GitHub Actions step summary if running in CI.
			if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
				_ = os.WriteFile(summaryPath, []byte(rep.GitHubSummary()), 0o644)
			}

			os.Exit(exitCode)
			return nil
		},
	}
	cmd.Flags().BoolVar(&reportFlag, "report", false, "write latexci-report.json")
	return cmd
}

func runBuild(cfg *config.Config) (*report.Report, int) {
	rep := report.New(cfg.Engine)
	c := compiler.New(cfg, slog.Default())

	if err := c.Build(rep); err != nil {
		slog.Error("build error", "err", err)
		rep.Finish("")
		return rep, 1
	}

	output := compiler.OutputPath(cfg)
	rep.Finish(output)

	switch {
	case rep.Errors > 0:
		return rep, 1
	case rep.Warnings > 0 && cfg.FailOnWarning:
		return rep, 2
	case rep.Warnings > 0:
		return rep, 2
	default:
		return rep, 0
	}
}

// ---------- watch ----------

func watchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch for file changes and recompile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			return watcher.Watch(".", func() error {
				rep, _ := runBuild(cfg)
				rep.Print()
				return nil
			}, slog.Default())
		},
	}
}

// ---------- clean ----------

func cleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove build artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			removed := 0
			err = filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				for _, x := range compiler.ArtifactExtensions {
					if ext == x {
						if rmErr := os.Remove(path); rmErr == nil {
							fmt.Printf("  removed %s\n", path)
							removed++
						}
						break
					}
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("walking directory: %w", err)
			}

			// Also remove output dir if empty.
			if entries, readErr := os.ReadDir(cfg.OutputDir); readErr == nil && len(entries) == 0 {
				_ = os.Remove(cfg.OutputDir)
			}

			fmt.Printf("\ncleaned %d file(s)\n", removed)
			return nil
		},
	}
}

// ---------- init ----------

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold a .latexci.yml in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Scaffold(config.DefaultConfigFile); err != nil {
				return err
			}
			fmt.Printf("created %s\n", config.DefaultConfigFile)
			return nil
		},
	}
}

// ---------- helpers ----------

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		slog.Warn("config not found, using defaults", "file", cfgFile)
		cfg = config.Defaults()
	}
	return cfg, nil
}
