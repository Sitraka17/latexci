package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sitrakaforler/latexci/internal/compiler"
	"github.com/sitrakaforler/latexci/internal/config"
	"github.com/sitrakaforler/latexci/internal/report"
	"github.com/sitrakaforler/latexci/internal/watcher"
)

// Set via -ldflags at build time.
var version = "dev"

var (
	cfgFile     string
	reportFlag  bool
	verboseFlag bool
)

func main() {
	root := &cobra.Command{
		Use:     "latexci",
		Short:   "Minimalist CI/CD pipeline for LaTeX projects",
		Long:    "latexci compiles LaTeX projects with structured error reporting and GitHub Actions support.",
		Version: version,
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
		newCmd(),
		openCmd(),
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
			fmt.Println("tip: run 'latexci new' to also create a starter main.tex")
			return nil
		},
	}
}

// ---------- new ----------

func newCmd() *cobra.Command {
	var engine string
	cmd := &cobra.Command{
		Use:   "new [directory]",
		Short: "Scaffold a complete LaTeX project (main.tex + .latexci.yml)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("creating directory %q: %w", dir, err)
				}
			}

			cfgPath := filepath.Join(dir, config.DefaultConfigFile)
			if err := config.ScaffoldWithEngine(cfgPath, engine); err != nil {
				return err
			}

			texPath := filepath.Join(dir, "main.tex")
			if _, err := os.Stat(texPath); err == nil {
				fmt.Printf("  exists   %s (skipped)\n", texPath)
			} else {
				if err := os.WriteFile(texPath, []byte(starterTeX(engine)), 0o644); err != nil {
					return fmt.Errorf("writing %s: %w", texPath, err)
				}
				fmt.Printf("  created  %s\n", texPath)
			}
			fmt.Printf("  created  %s\n", cfgPath)
			fmt.Printf("\nready — run: latexci build\n")
			return nil
		},
	}
	cmd.Flags().StringVarP(&engine, "engine", "e", "pdflatex", "LaTeX engine: pdflatex | xelatex | lualatex")
	return cmd
}

func starterTeX(engine string) string {
	fontPkg := ""
	if engine == "xelatex" || engine == "lualatex" {
		fontPkg = "\n\\usepackage{fontspec}"
	}
	return fmt.Sprintf(`\documentclass{article}
\usepackage[utf8]{inputenc}
\usepackage[T1]{fontenc}%s
\usepackage{hyperref}

\title{My Document}
\author{Author}
\date{\today}

\begin{document}

\maketitle

\section{Introduction}

Hello, \LaTeX!

\end{document}
`, fontPkg)
}

// ---------- open ----------

func openCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open the compiled PDF with the system viewer",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			pdf := compiler.OutputPath(cfg)
			if _, err := os.Stat(pdf); err != nil {
				return fmt.Errorf("PDF not found at %q — run 'latexci build' first", pdf)
			}
			return openPDF(pdf)
		},
	}
}

func openPDF(path string) error {
	var openCmd string
	switch runtime.GOOS {
	case "darwin":
		openCmd = "open"
	case "linux":
		openCmd = "xdg-open"
	case "windows":
		openCmd = "start"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	cmd := exec.Command(openCmd, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
