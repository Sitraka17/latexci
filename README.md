# latexci

![Build](https://img.shields.io/github/actions/workflow/status/sitrakaforler/latexci/ci.yml?label=build)
![Go version](https://img.shields.io/badge/go-1.22%2B-00ADD8)
![License](https://img.shields.io/badge/license-MIT-green)
![Release](https://img.shields.io/github/v/release/sitrakaforler/latexci)

**Minimalist CI/CD pipeline for LaTeX projects** — compile, watch, clean, and publish PDFs from a single binary. Works locally and on GitHub Actions.

---

## Quick start

```bash
# 1. Install
go install github.com/sitrakaforler/latexci/cmd/latexci@latest

# 2. Scaffold config
latexci init

# 3. Build your PDF
latexci build
```

Your PDF lands in `build/main.pdf`.

---

## CLI commands

| Command | Description |
|---|---|
| `latexci build` | Compile the project (single or multi-pass) |
| `latexci watch` | Watch mode — recompile on `.tex` / `.bib` save |
| `latexci clean` | Remove build artifacts (`.aux`, `.log`, `.toc` …) |
| `latexci init` | Scaffold `.latexci.yml` in current directory |

Global flags: `--config <file>` · `--verbose` · `--report`

---

## `.latexci.yml` reference

| Key | Type | Default | Description |
|---|---|---|---|
| `engine` | string | `pdflatex` | LaTeX engine: `pdflatex` \| `xelatex` \| `lualatex` |
| `main` | string | `main.tex` | Entry-point `.tex` file |
| `output_dir` | string | `build/` | Directory for generated files |
| `runs` | int | `2` | Compilation passes (needed for TOC, cross-refs) |
| `bibliography` | bool | `false` | Run `biber`/`bibtex` after first pass |
| `draft` | bool | `false` | Enable `-draftmode` (faster, no PDF output) |
| `fail_on_warning` | bool | `false` | Exit code 2 when any warning is present |
| `artifacts` | list | `build/*.pdf` | Globs for GitHub Actions artifact upload |

---

## GitHub Actions usage

Copy-paste this workflow into `.github/workflows/latex.yml`:

```yaml
name: Build PDF

on:
  push:
    branches: [main]
  pull_request:

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Compile LaTeX
        uses: sitrakaforler/latexci@v1
        with:
          engine: pdflatex
          main-file: main.tex
          output-dir: build
          runs: 2
          bibliography: false
          fail-on-warning: false
```

The action will:
1. Install TeX Live if not present
2. Compile your project with the configured engine
3. Upload the PDF as a GitHub Actions artifact
4. Post a build summary table in the step summary

---

## Error output format

Errors are printed in Go-compiler style for easy IDE integration:

```
./chapters/intro.tex:42: undefined control sequence \maketitle
./main.tex:0: LaTeX Warning: Citation 'foo' on page 3 undefined
```

Exit codes: `0` = success · `1` = errors · `2` = warnings only

---

## Build report (`--report`)

```bash
latexci build --report
```

Writes `latexci-report.json`:

```json
{
  "status": "success",
  "engine": "xelatex",
  "passes": 2,
  "duration_ms": 3420,
  "warnings": 3,
  "errors": 0,
  "pages": 12,
  "output": "build/main.pdf"
}
```

---

## Watch mode

```bash
latexci watch
```

- Watches `.tex`, `.bib`, `.sty`, `.cls` recursively
- Debounces 500ms after last change before recompiling
- Prints colored error diffs between builds

---

## Comparison

| Feature | **latexci** | latexmk | tectonic | arara |
|---|---|---|---|---|
| Single binary | ✓ | ✗ | ✓ | ✗ |
| GitHub Actions action | ✓ | ✗ | ✗ | ✗ |
| JSON build report | ✓ | ✗ | ✗ | ✗ |
| Watch mode | ✓ | ✓ | ✗ | ✗ |
| Structured error parsing | ✓ | partial | ✗ | ✗ |
| Cross-platform binary | ✓ | ✗ | ✓ | ✗ |
| Zero runtime deps | ✓ | ✗ | ✓ | ✗ |
| Config file | ✓ | ✓ | ✗ | ✓ |

---

## Development

```bash
# Build
make build

# Run tests
make test

# Lint
make lint

# Cross-platform release (requires goreleaser)
make release-snapshot
```

## License

MIT © sitrakaforler
