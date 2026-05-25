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
		Use:          "latexci",
		Short:        "Minimalist CI/CD pipeline for LaTeX projects",
		Long:         "latexci compiles LaTeX projects with structured error reporting and GitHub Actions support.",
		Version:      version,
		SilenceUsage: true, // don't print usage on runtime errors
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
		doctorCmd(),
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

// ---------- doctor ----------

type checkResult struct {
	label  string
	ok     bool
	detail string
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that your environment is ready to compile LaTeX",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadConfig()
			checks := runDoctor(cfg)

			allOK := true
			for _, c := range checks {
				icon := "✓"
				if !c.ok {
					icon = "✗"
					allOK = false
				}
				colored := colorCheck(icon, c.ok)
				if c.detail != "" {
					fmt.Printf("  %s  %-30s %s\n", colored, c.label, c.detail)
				} else {
					fmt.Printf("  %s  %s\n", colored, c.label)
				}
			}
			fmt.Println()
			if allOK {
				fmt.Println(colorCheckGreen("All checks passed — ready to compile."))
			} else {
				fmt.Println(colorCheckRed("Some checks failed. Fix the issues above, then run 'latexci build'."))
				return fmt.Errorf("environment not ready")
			}
			return nil
		},
	}
}

func runDoctor(cfg *config.Config) []checkResult {
	var results []checkResult

	add := func(label string, ok bool, detail string) {
		results = append(results, checkResult{label, ok, detail})
	}

	// 1. Config file
	_, cfgErr := os.Stat(cfgFile)
	if cfgErr == nil {
		add(".latexci.yml present", true, cfgFile)
	} else {
		add(".latexci.yml present", false, "run 'latexci init' to create one")
	}

	// 2. Main .tex file
	_, texErr := os.Stat(cfg.Main)
	if texErr == nil {
		add("main file exists", true, cfg.Main)
	} else {
		add("main file exists", false, fmt.Sprintf("%q not found — run 'latexci new'", cfg.Main))
	}

	// 3. LaTeX engine
	engines := []string{cfg.Engine, "pdflatex", "xelatex", "lualatex"}
	seen := map[string]bool{}
	for _, e := range engines {
		if seen[e] {
			continue
		}
		seen[e] = true
		if p, err := exec.LookPath(e); err == nil {
			add(fmt.Sprintf("engine %s", e), true, p)
		} else {
			add(fmt.Sprintf("engine %s", e), false, "not found in PATH")
		}
	}

	// 4. biber / bibtex (only if bibliography enabled)
	if cfg.Bibliography {
		if p, err := exec.LookPath("biber"); err == nil {
			add("biber (bibliography)", true, p)
		} else if p, err := exec.LookPath("bibtex"); err == nil {
			add("bibtex (bibliography)", true, p)
		} else {
			add("biber/bibtex (bibliography)", false, "install texlive-bibtex-extra or biber")
		}
	}

	// 5. Output dir writable
	outDir := cfg.OutputDir
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		add("output dir writable", false, fmt.Sprintf("cannot create %q: %v", outDir, err))
	} else {
		add("output dir writable", true, outDir)
	}

	// 6. Disk space (warn if < 100 MB free in current dir)
	if free, err := diskFreeBytes("."); err == nil {
		mb := free / 1024 / 1024
		if mb < 100 {
			add("disk space", false, fmt.Sprintf("only %d MB free — LaTeX build artifacts can be large", mb))
		} else {
			add("disk space", true, fmt.Sprintf("%d MB free", mb))
		}
	}

	return results
}

func colorCheck(icon string, ok bool) string {
	noColor := os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
	if noColor {
		return icon
	}
	if ok {
		return "\033[32m" + icon + "\033[0m"
	}
	return "\033[31m" + icon + "\033[0m"
}

func colorCheckGreen(s string) string {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func colorCheckRed(s string) string {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return s
	}
	return "\033[31m" + s + "\033[0m"
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
