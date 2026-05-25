package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
	StatusWarning Status = "warning"
)

type LaTeXError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
	Kind    string `json:"kind"` // "error" | "warning" | "overfull"
}

type Report struct {
	Status      Status       `json:"status"`
	Engine      string       `json:"engine"`
	Passes      int          `json:"passes"`
	DurationMs  int64        `json:"duration_ms"`
	Warnings    int          `json:"warnings"`
	Errors      int          `json:"errors"`
	Pages       int          `json:"pages"`
	Output      string       `json:"output"`
	Details     []LaTeXError `json:"details,omitempty"`
	startedAt   time.Time
}

func New(engine string) *Report {
	return &Report{Engine: engine, startedAt: time.Now()}
}

func (r *Report) Finish(output string) {
	r.DurationMs = time.Since(r.startedAt).Milliseconds()
	r.Output = output
	switch {
	case r.Errors > 0:
		r.Status = StatusError
	case r.Warnings > 0:
		r.Status = StatusWarning
	default:
		r.Status = StatusSuccess
	}
}

func (r *Report) WriteJSON(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing report %q: %w", path, err)
	}
	return nil
}

// Print prints a human-readable summary to stdout using ANSI colours.
func (r *Report) Print() {
	statusIcon := colorGreen("✓")
	if r.Status == StatusError {
		statusIcon = colorRed("✗")
	} else if r.Status == StatusWarning {
		statusIcon = colorYellow("⚠")
	}

	fmt.Printf("\n%s  latexci build %s  [%s]\n", statusIcon, string(r.Status), r.Engine)
	fmt.Printf("   passes : %d\n", r.Passes)
	fmt.Printf("   pages  : %d\n", r.Pages)
	fmt.Printf("   errors : %s\n", countColored(r.Errors, colorRed))
	fmt.Printf("   warns  : %s\n", countColored(r.Warnings, colorYellow))
	fmt.Printf("   time   : %dms\n", r.DurationMs)
	if r.Output != "" {
		fmt.Printf("   output : %s\n", r.Output)
	}
	fmt.Println()

	for _, d := range r.Details {
		prefix := colorRed("error")
		if d.Kind == "warning" {
			prefix = colorYellow("warn ")
		} else if d.Kind == "overfull" {
			prefix = colorYellow("oflow")
		}
		loc := d.File
		if d.Line > 0 {
			loc = fmt.Sprintf("%s:%d", d.File, d.Line)
		}
		fmt.Printf("  %s  %s: %s\n", prefix, loc, d.Message)
	}
}

// GitHubSummary returns a Markdown table suitable for $GITHUB_STEP_SUMMARY.
func (r *Report) GitHubSummary() string {
	var sb strings.Builder
	sb.WriteString("## latexci Build Summary\n\n")
	sb.WriteString("| Key | Value |\n|---|---|\n")
	sb.WriteString(fmt.Sprintf("| Status | %s |\n", string(r.Status)))
	sb.WriteString(fmt.Sprintf("| Engine | `%s` |\n", r.Engine))
	sb.WriteString(fmt.Sprintf("| Passes | %d |\n", r.Passes))
	sb.WriteString(fmt.Sprintf("| Pages | %d |\n", r.Pages))
	sb.WriteString(fmt.Sprintf("| Errors | %d |\n", r.Errors))
	sb.WriteString(fmt.Sprintf("| Warnings | %d |\n", r.Warnings))
	sb.WriteString(fmt.Sprintf("| Duration | %dms |\n", r.DurationMs))
	sb.WriteString(fmt.Sprintf("| Output | `%s` |\n", r.Output))
	return sb.String()
}

func colorRed(s string) string    { return "\033[31m" + s + "\033[0m" }
func colorGreen(s string) string  { return "\033[32m" + s + "\033[0m" }
func colorYellow(s string) string { return "\033[33m" + s + "\033[0m" }

func countColored(n int, color func(string) string) string {
	s := fmt.Sprintf("%d", n)
	if n > 0 {
		return color(s)
	}
	return s
}
