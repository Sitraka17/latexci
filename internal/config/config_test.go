package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".latexci.yml")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Engine != "pdflatex" {
		t.Errorf("Engine = %q, want pdflatex", cfg.Engine)
	}
	if cfg.Main != "main.tex" {
		t.Errorf("Main = %q, want main.tex", cfg.Main)
	}
	if cfg.Runs != 2 {
		t.Errorf("Runs = %d, want 2", cfg.Runs)
	}
}

func TestLoadOverrides(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".latexci.yml")
	content := "engine: xelatex\nmain: thesis.tex\nruns: 3\nbibliography: true\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Engine != "xelatex" {
		t.Errorf("Engine = %q, want xelatex", cfg.Engine)
	}
	if cfg.Main != "thesis.tex" {
		t.Errorf("Main = %q, want thesis.tex", cfg.Main)
	}
	if cfg.Runs != 3 {
		t.Errorf("Runs = %d, want 3", cfg.Runs)
	}
	if !cfg.Bibliography {
		t.Error("Bibliography should be true")
	}
}

func TestScaffold(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".latexci.yml")

	if err := Scaffold(path); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	// Second call must fail (file exists).
	if err := Scaffold(path); err == nil {
		t.Error("expected error on second Scaffold call")
	}
	// The scaffolded file must be valid YAML.
	if _, err := Load(path); err != nil {
		t.Errorf("Load(scaffolded): %v", err)
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("LATEXCI_ENGINE", "lualatex")
	t.Setenv("LATEXCI_MAIN", "paper.tex")
	t.Setenv("LATEXCI_OUTPUT_DIR", "out")
	t.Setenv("LATEXCI_RUNS", "4")
	t.Setenv("LATEXCI_FAIL_ON_WARNING", "true")
	t.Setenv("LATEXCI_BIBLIOGRAPHY", "true")
	t.Setenv("LATEXCI_TIMEOUT", "120")

	tmp := t.TempDir()
	path := filepath.Join(tmp, ".latexci.yml")
	// Env must win over the file.
	content := "engine: xelatex\nmain: thesis.tex\nruns: 3\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Engine != "lualatex" {
		t.Errorf("Engine = %q, want lualatex", cfg.Engine)
	}
	if cfg.Main != "paper.tex" {
		t.Errorf("Main = %q, want paper.tex", cfg.Main)
	}
	if cfg.OutputDir != "out" {
		t.Errorf("OutputDir = %q, want out", cfg.OutputDir)
	}
	if cfg.Runs != 4 {
		t.Errorf("Runs = %d, want 4", cfg.Runs)
	}
	if !cfg.FailOnWarning || !cfg.Bibliography {
		t.Error("FailOnWarning and Bibliography should be true")
	}
	if cfg.TimeoutSeconds != 120 {
		t.Errorf("TimeoutSeconds = %d, want 120", cfg.TimeoutSeconds)
	}
}

func TestEnvInvalidValuesIgnored(t *testing.T) {
	t.Setenv("LATEXCI_RUNS", "not-a-number")
	t.Setenv("LATEXCI_FAIL_ON_WARNING", "maybe")
	cfg := Defaults()
	if cfg.Runs != 2 {
		t.Errorf("Runs = %d, want default 2", cfg.Runs)
	}
	if cfg.FailOnWarning {
		t.Error("FailOnWarning should stay false on invalid env value")
	}
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Engine == "" || cfg.Main == "" || cfg.Runs <= 0 {
		t.Errorf("Defaults() returned zero values: %+v", cfg)
	}
}
