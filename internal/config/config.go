package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = ".latexci.yml"

type Config struct {
	Engine         string   `yaml:"engine"`
	Main           string   `yaml:"main"`
	OutputDir      string   `yaml:"output_dir"`
	Runs           int      `yaml:"runs"`
	Bibliography   bool     `yaml:"bibliography"`
	Draft          bool     `yaml:"draft"`
	FailOnWarning  bool     `yaml:"fail_on_warning"`
	TimeoutSeconds int      `yaml:"timeout"` // max seconds per engine pass (0 = no limit)
	Artifacts      []string `yaml:"artifacts"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}
	cfg.applyEnv()
	cfg.applyDefaults()
	return &cfg, nil
}

// Defaults returns a Config with all defaults applied.
func Defaults() *Config {
	c := &Config{}
	c.applyEnv()
	c.applyDefaults()
	return c
}

// applyEnv overrides config values from LATEXCI_* environment variables.
// This is how the GitHub Action passes its inputs to the CLI.
func (c *Config) applyEnv() {
	if v := os.Getenv("LATEXCI_ENGINE"); v != "" {
		c.Engine = v
	}
	if v := os.Getenv("LATEXCI_MAIN"); v != "" {
		c.Main = v
	}
	if v := os.Getenv("LATEXCI_OUTPUT_DIR"); v != "" {
		c.OutputDir = v
	}
	if v := os.Getenv("LATEXCI_RUNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.Runs = n
		}
	}
	if v := os.Getenv("LATEXCI_FAIL_ON_WARNING"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.FailOnWarning = b
		}
	}
	if v := os.Getenv("LATEXCI_BIBLIOGRAPHY"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.Bibliography = b
		}
	}
	if v := os.Getenv("LATEXCI_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.TimeoutSeconds = n
		}
	}
}

func (c *Config) applyDefaults() {
	if c.Engine == "" {
		c.Engine = "pdflatex"
	}
	if c.Main == "" {
		c.Main = "main.tex"
	}
	if c.OutputDir == "" {
		c.OutputDir = "build"
	}
	if c.Runs <= 0 {
		c.Runs = 2
	}
}

func Scaffold(path string) error {
	return ScaffoldWithEngine(path, "pdflatex")
}

func ScaffoldWithEngine(path, engine string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%q already exists", path)
	}
	content := fmt.Sprintf(`engine: %s        # pdflatex | xelatex | lualatex
main: main.tex          # LaTeX entry point
output_dir: build/
runs: 2                 # compilation passes (for TOC, refs)
bibliography: false     # run bibtex/biber after first pass
draft: false
fail_on_warning: false
timeout: 0              # max seconds per compilation pass (0 = no limit)
artifacts:
  - build/*.pdf
`, engine)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}
