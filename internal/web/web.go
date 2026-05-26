// Package web converts LaTeX documents to self-contained HTML websites
// using pandoc, then prepares the output for GitHub Pages deployment.
package web

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// Options controls HTML generation.
type Options struct {
	Main      string // source .tex file
	OutputDir string // where to write build/
	Title     string // overrides extracted title
	Theme     string // "light" | "dark" | "academic"
	TOC       bool   // include table of contents
	SelfContained bool // embed all assets in single HTML file
}

// Build converts the LaTeX source to HTML.
// Returns the path to the generated index.html.
func Build(opts Options) (string, error) {
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	pandoc, err := exec.LookPath("pandoc")
	if err != nil {
		return "", fmt.Errorf("pandoc not found in PATH — install it with: brew install pandoc\n" +
			"  or: https://pandoc.org/installing.html")
	}

	// Write the CSS theme into a temp file so pandoc can embed it.
	cssFile, err := os.CreateTemp("", "latexci-*.css")
	if err != nil {
		return "", fmt.Errorf("creating temp css: %w", err)
	}
	defer os.Remove(cssFile.Name())
	if _, err := cssFile.WriteString(themeCSS(opts.Theme)); err != nil {
		return "", fmt.Errorf("writing css: %w", err)
	}
	cssFile.Close()

	outPath := filepath.Join(filepath.Clean(opts.OutputDir), "index.html")

	args := []string{
		opts.Main,
		"--from", "latex",
		"--to", "html5",
		"--output", outPath,
		"--css", cssFile.Name(),
		"--mathjax",                       // render math via MathJax CDN
		"--syntax-highlighting", "pygments",
		"--metadata", "lang=en",
	}
	if opts.TOC {
		args = append(args, "--toc", "--toc-depth=3")
	}
	if opts.Title != "" {
		args = append(args, "--metadata", "title="+opts.Title)
	}
	if opts.SelfContained {
		args = append(args, "--embed-resources", "--standalone")
	} else {
		args = append(args, "--standalone")
	}

	cmd := exec.Command(pandoc, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pandoc conversion failed: %w", err)
	}

	// Inject our custom CSS inline so it works even without the temp file.
	if err := injectCSS(outPath, opts.Theme); err != nil {
		fmt.Fprintf(os.Stderr, "warning: CSS injection failed: %v\n", err)
	}

	// Write GitHub Pages config files.
	if err := writeGitHubPagesFiles(opts.OutputDir, opts.Main, opts.Title); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write GitHub Pages files: %v\n", err)
	}

	return outPath, nil
}

// writeGitHubPagesFiles creates the supporting files needed for GitHub Pages.
func writeGitHubPagesFiles(dir, mainTex, title string) error {
	// .nojekyll tells GitHub Pages not to run Jekyll processing.
	if err := os.WriteFile(filepath.Join(dir, ".nojekyll"), []byte(""), 0o644); err != nil {
		return err
	}

	// _config.yml for optional Jekyll fallback.
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(mainTex), ".tex")
	}
	config := fmt.Sprintf("title: %q\ntheme: minima\n", title)
	return os.WriteFile(filepath.Join(dir, "_config.yml"), []byte(config), 0o644)
}

// injectCSS rewrites the HTML to include the theme CSS inline
// (pandoc references the temp file path which won't exist in production).
func injectCSS(htmlPath, theme string) error {
	data, err := os.ReadFile(htmlPath)
	if err != nil {
		return err
	}
	css := themeCSS(theme)
	styleTag := "<style>\n" + css + "\n</style>"
	// Replace the external CSS link with inline styles.
	content := string(data)
	// Find and replace the <link rel="stylesheet" ...> line referencing our temp file.
	lines := strings.Split(content, "\n")
	var out []string
	injected := false
	for _, line := range lines {
		if !injected && strings.Contains(line, `rel="stylesheet"`) && strings.Contains(line, ".css") {
			out = append(out, styleTag)
			injected = true
			continue // drop the external link
		}
		out = append(out, line)
	}
	return os.WriteFile(htmlPath, []byte(strings.Join(out, "\n")), 0o644)
}

func themeCSS(theme string) string {
	base := baseCSS()
	switch theme {
	case "dark":
		return base + darkOverride()
	case "academic":
		return base + academicOverride()
	default: // "light"
		return base
	}
}

func baseCSS() string {
	return `/* latexci generated stylesheet */
:root {
  --bg: #ffffff;
  --fg: #1a1a2e;
  --accent: #1a7f7a;
  --accent-light: #e8f5f4;
  --code-bg: #f4f4f5;
  --border: #e2e8f0;
  --max-width: 860px;
  --font-serif: 'Georgia', 'Times New Roman', serif;
  --font-sans: 'Inter', 'Helvetica Neue', sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
}
*, *::before, *::after { box-sizing: border-box; }
body {
  background: var(--bg);
  color: var(--fg);
  font-family: var(--font-serif);
  font-size: 1.05rem;
  line-height: 1.75;
  max-width: var(--max-width);
  margin: 0 auto;
  padding: 2rem 1.5rem 4rem;
}
h1, h2, h3, h4 {
  font-family: var(--font-sans);
  font-weight: 700;
  line-height: 1.25;
  margin-top: 2.5rem;
  color: var(--fg);
}
h1 { font-size: 2.2rem; border-bottom: 3px solid var(--accent); padding-bottom: .4rem; }
h2 { font-size: 1.5rem; border-left: 4px solid var(--accent); padding-left: .75rem; }
h3 { font-size: 1.15rem; }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
code, pre {
  font-family: var(--font-mono);
  font-size: .9rem;
  background: var(--code-bg);
  border-radius: 4px;
}
code { padding: .15em .4em; }
pre { padding: 1rem 1.25rem; overflow-x: auto; border: 1px solid var(--border); }
pre code { background: none; padding: 0; }
blockquote {
  border-left: 4px solid var(--accent);
  background: var(--accent-light);
  margin: 1.5rem 0;
  padding: .75rem 1.25rem;
  border-radius: 0 6px 6px 0;
}
table {
  width: 100%;
  border-collapse: collapse;
  margin: 1.5rem 0;
  font-size: .95rem;
}
th {
  background: var(--accent);
  color: #fff;
  font-family: var(--font-sans);
  padding: .6rem 1rem;
  text-align: left;
}
td { padding: .55rem 1rem; border-bottom: 1px solid var(--border); }
tr:nth-child(even) td { background: var(--accent-light); }
img { max-width: 100%; height: auto; border-radius: 6px; }
#TOC {
  background: var(--accent-light);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1rem 1.5rem;
  margin-bottom: 2rem;
  font-family: var(--font-sans);
  font-size: .92rem;
}
#TOC ul { padding-left: 1.2rem; }
#TOC > ul { padding-left: 0; list-style: none; }
header { margin-bottom: 3rem; }
.author, .date { color: #666; font-family: var(--font-sans); font-size: .95rem; }
`
}

func darkOverride() string {
	return `
:root {
  --bg: #0f1117;
  --fg: #e2e8f0;
  --accent: #38b2ac;
  --accent-light: #1a2a2a;
  --code-bg: #1e2130;
  --border: #2d3748;
}
`
}

func academicOverride() string {
	return `
:root {
  --bg: #fffef8;
  --fg: #2d2d2d;
  --accent: #8b0000;
  --accent-light: #fff5f5;
  --max-width: 750px;
}
body { font-size: 1.0rem; line-height: 1.8; }
h1 { font-size: 1.8rem; }
`
}

// DeployInstructions returns a printable guide for publishing to GitHub Pages.
func DeployInstructions(outputDir, repoURL string) string {
	t := template.Must(template.New("").Parse(`
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  🌐  Publish to GitHub Pages (free hosting)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Push your project to GitHub (if not already done):

     git init && git add . && git commit -m "initial"
     git remote add origin <your-repo-url>
     git push -u origin main

2. In your GitHub repo → Settings → Pages:
     Source: "GitHub Actions"   ← choose this

3. Create  .github/workflows/pages.yml  (already written for you below)

4. git add . && git commit -m "deploy website" && git push

5. Your site will be live at:
     https://<your-username>.github.io/<repo-name>/

The HTML is in:  {{.OutputDir}}/index.html
`))
	var sb strings.Builder
	_ = t.Execute(&sb, map[string]string{"OutputDir": outputDir})
	return sb.String()
}

// WriteGitHubPagesWorkflow writes the GitHub Actions workflow for Pages deployment.
func WriteGitHubPagesWorkflow(projectRoot string) error {
	dir := filepath.Join(projectRoot, ".github", "workflows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "pages.yml")
	return os.WriteFile(path, []byte(pagesWorkflow()), 0o644)
}

func pagesWorkflow() string {
	return `name: Deploy Website

on:
  push:
    branches: [main]
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: true

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install pandoc
        run: |
          wget -q https://github.com/jgm/pandoc/releases/download/3.6/pandoc-3.6-linux-amd64.tar.gz
          tar xf pandoc-*.tar.gz
          sudo mv pandoc-*/bin/pandoc /usr/local/bin/

      - name: Install latexci
        run: go install github.com/Sitraka17/latexci/cmd/latexci@latest

      - name: Build website
        run: latexci web --toc --theme academic

      - name: Upload Pages artifact
        uses: actions/upload-pages-artifact@v3
        with:
          path: build/

  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
`
}
