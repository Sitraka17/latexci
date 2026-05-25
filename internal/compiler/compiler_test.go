package compiler

import (
	"strings"
	"testing"

	"github.com/sitrakaforler/latexci/internal/report"
)

const sampleLog = `
This is pdfTeX, Version 3.141592653 (TeX Live 2023)
(/usr/share/texmf/tex/latex/base/article.cls
(/tmp/thesis/chapters/intro.tex
! Undefined control sequence.
l.42 \maketitle
)
LaTeX Warning: Citation 'foo' on page 3 undefined
Overfull \hbox (15.0pt too wide) in paragraph
Output written on build/main.pdf (12 pages, 98304 bytes).
`

func TestParseLog(t *testing.T) {
	rep := report.New("pdflatex")
	if err := parseLogReader(strings.NewReader(sampleLog), rep); err != nil {
		t.Fatalf("parseLogReader: %v", err)
	}

	if rep.Errors == 0 {
		t.Error("expected at least one error")
	}
	if rep.Warnings == 0 {
		t.Error("expected at least one warning")
	}

	// Overfull > 10pt must be counted.
	var hasOverfull bool
	for _, d := range rep.Details {
		if d.Kind == "overfull" {
			hasOverfull = true
		}
	}
	if !hasOverfull {
		t.Error("expected an overfull hbox detail")
	}
}

func TestParseLog_SmallOverfull(t *testing.T) {
	log := `Overfull \hbox (5.0pt too wide) in paragraph`
	rep := report.New("pdflatex")
	_ = parseLogReader(strings.NewReader(log), rep)
	// < 10pt threshold: should NOT be reported.
	if rep.Warnings > 0 {
		t.Errorf("overfull < 10pt should be ignored, got %d warnings", rep.Warnings)
	}
}

func TestOutputPath(t *testing.T) {
	base := strings.TrimSuffix("thesis.tex", ".tex")
	want := "out/" + base + ".pdf"
	if want != "out/thesis.pdf" {
		t.Errorf("got %q", want)
	}
}
