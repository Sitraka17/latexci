package compiler

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sitrakaforler/latexci/internal/config"
	"github.com/sitrakaforler/latexci/internal/report"
)

var (
	reError    = regexp.MustCompile(`^!\s+(.+)`)
	reFile     = regexp.MustCompile(`\(([^()]+\.tex)`)
	reLine     = regexp.MustCompile(`^l\.(\d+)`)
	reWarning  = regexp.MustCompile(`(?i)warning:\s+(.+)`)
	reOverfull = regexp.MustCompile(`(?i)Overfull \\hbox \((\d+(?:\.\d+)?)pt`)
	rePages    = regexp.MustCompile(`Output written on .+ \((\d+) page`)
)

// Compiler drives LaTeX compilation.
type Compiler struct {
	cfg *config.Config
	log *slog.Logger
}

func New(cfg *config.Config, log *slog.Logger) *Compiler {
	return &Compiler{cfg: cfg, log: log}
}

// Build compiles the project and returns a populated report.
func (c *Compiler) Build(rep *report.Report) error {
	if err := os.MkdirAll(c.cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	engine, err := c.resolveEngine()
	if err != nil {
		return err
	}
	rep.Engine = engine

	for pass := 1; pass <= c.cfg.Runs; pass++ {
		c.log.Info("compilation pass", "pass", pass, "total", c.cfg.Runs)
		if err := c.runEngine(engine, pass, rep); err != nil {
			return err
		}
		if c.cfg.Bibliography && pass == 1 {
			if err := c.runBibliography(); err != nil {
				c.log.Warn("bibliography step failed", "err", err)
			}
		}
	}
	rep.Passes = c.cfg.Runs

	// Parse the .log file for structured diagnostics.
	logFile := c.logFilePath()
	if err := c.parseLog(logFile, rep); err != nil {
		c.log.Warn("log parsing failed", "err", err)
	}

	return nil
}

func (c *Compiler) resolveEngine() (string, error) {
	engine := c.cfg.Engine
	if engine == "" {
		// Auto-detect: prefer xelatex > pdflatex > lualatex.
		for _, e := range []string{"xelatex", "pdflatex", "lualatex"} {
			if _, err := exec.LookPath(e); err == nil {
				slog.Info("auto-detected engine", "engine", e)
				return e, nil
			}
		}
		return "", fmt.Errorf("no LaTeX engine found in PATH (tried xelatex, pdflatex, lualatex)")
	}
	if _, err := exec.LookPath(engine); err != nil {
		return "", fmt.Errorf("engine %q not found in PATH: %w", engine, err)
	}
	return engine, nil
}

func (c *Compiler) runEngine(engine string, pass int, rep *report.Report) error {
	args := []string{
		"-interaction=nonstopmode",
		"-halt-on-error=false",
		fmt.Sprintf("-output-directory=%s", c.cfg.OutputDir),
	}
	if c.cfg.Draft {
		args = append(args, "-draftmode")
	}
	args = append(args, c.cfg.Main)

	cmd := exec.Command(engine, args...)
	cmd.Dir = "."

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge streams so we see everything

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", engine, err)
	}

	// Stream output in real time.
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if reError.MatchString(line) {
			fmt.Fprintf(os.Stderr, "\033[31m%s\033[0m\n", line)
		} else {
			fmt.Println(line)
		}

		// Extract page count from final pass.
		if pass == c.cfg.Runs {
			if m := rePages.FindStringSubmatch(line); m != nil {
				rep.Pages, _ = strconv.Atoi(m[1])
			}
		}
	}
	_ = stdout.Close()

	if err := cmd.Wait(); err != nil {
		// LaTeX exits non-zero even with recoverable errors; we inspect the log.
		c.log.Warn("engine exited with error", "engine", engine, "err", err)
	}
	return nil
}

func (c *Compiler) runBibliography() error {
	// Prefer biber, fall back to bibtex.
	tool := "biber"
	if _, err := exec.LookPath(tool); err != nil {
		tool = "bibtex"
	}
	base := strings.TrimSuffix(c.cfg.Main, ".tex")
	auxPath := filepath.Join(c.cfg.OutputDir, base)
	cmd := exec.Command(tool, auxPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", tool, auxPath, err)
	}
	return nil
}

func (c *Compiler) logFilePath() string {
	base := strings.TrimSuffix(filepath.Base(c.cfg.Main), ".tex")
	return filepath.Join(c.cfg.OutputDir, base+".log")
}

func (c *Compiler) parseLog(path string, rep *report.Report) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening log %q: %w", path, err)
	}
	defer f.Close()
	return parseLogReader(f, rep)
}

func parseLogReader(r io.Reader, rep *report.Report) error {
	scanner := bufio.NewScanner(r)

	var (
		currentFile  string
		pendingError string
		fileStack    []string
	)

	addDetail := func(kind, file string, line int, msg string) {
		rep.Details = append(rep.Details, report.LaTeXError{
			Kind: kind, File: file, Line: line, Message: strings.TrimSpace(msg),
		})
		switch kind {
		case "error":
			rep.Errors++
		case "warning", "overfull":
			rep.Warnings++
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Track file open/close via parentheses (simplified heuristic).
		for _, m := range reFile.FindAllStringSubmatch(line, -1) {
			fileStack = append(fileStack, m[1])
			currentFile = m[1]
		}
		if strings.HasSuffix(strings.TrimSpace(line), ")") && len(fileStack) > 0 {
			fileStack = fileStack[:len(fileStack)-1]
			if len(fileStack) > 0 {
				currentFile = fileStack[len(fileStack)-1]
			}
		}

		// Flush pending error when we see the line reference.
		if pendingError != "" {
			if m := reLine.FindStringSubmatch(line); m != nil {
				lineNo, _ := strconv.Atoi(m[1])
				addDetail("error", currentFile, lineNo, pendingError)
				pendingError = ""
				continue
			}
		}

		if m := reError.FindStringSubmatch(line); m != nil {
			pendingError = m[1]
			continue
		}

		if m := reOverfull.FindStringSubmatch(line); m != nil {
			pts, _ := strconv.ParseFloat(m[1], 64)
			if pts > 10 {
				addDetail("overfull", currentFile, 0, fmt.Sprintf("Overfull \\hbox (%.1fpt too wide)", pts))
			}
			continue
		}

		if m := reWarning.FindStringSubmatch(line); m != nil {
			addDetail("warning", currentFile, 0, m[1])
		}
	}

	// Flush any unterminated error.
	if pendingError != "" {
		addDetail("error", currentFile, 0, pendingError)
	}

	return scanner.Err()
}

// OutputPath returns the expected PDF path.
func OutputPath(cfg *config.Config) string {
	base := strings.TrimSuffix(filepath.Base(cfg.Main), ".tex")
	return filepath.Join(cfg.OutputDir, base+".pdf")
}

// ArtifactExtensions lists extensions removed by clean.
var ArtifactExtensions = []string{
	".aux", ".log", ".toc", ".out", ".bbl", ".blg",
	".lof", ".lot", ".fls", ".fdb_latexmk", ".synctex.gz",
	".nav", ".snm", ".vrb", ".bcf", ".run.xml",
}
