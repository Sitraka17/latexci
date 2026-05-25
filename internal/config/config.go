package config

import (
	"fmt"
	"os"

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
	cfg.applyDefaults()
	return &cfg, nil
}

// Defaults returns a Config with all defaults applied.
func Defaults() *Config {
	c := &Config{}
	c.applyDefaults()
	return c
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
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%q already exists", path)
	}
	content := `engine: pdflatex        # pdflatex | xelatex | lualatex
main: main.tex          # LaTeX entry point
output_dir: build/
runs: 2                 # compilation passes (for TOC, refs)
bibliography: false     # run bibtex/biber after first pass
draft: false
fail_on_warning: false
artifacts:
  - build/*.pdf
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}
