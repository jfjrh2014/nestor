// Package ui provides styled output helpers for nestor.
package ui

import (
	"fmt"
	"io"
)

// Styles — kept simple and dependency-free for now.
// We'll swap these for lipgloss once we have Phase 1 functionality.
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
)

// Printer writes styled lines to a writer.
type Printer struct {
	w  io.Writer
}

// New returns a Printer writing to w.
func New(w io.Writer) *Printer {
	return &Printer{w: w}
}

// Step prints a top-level step: "nestor: ✓ detecting platform..."
func (p *Printer) Step(icon, msg string) {
	fmt.Fprintf(p.w, "%s%s%s nestor: %s\n", colorFor(icon), icon, reset, msg)
}

// OK prints a success line.
func (p *Printer) OK(msg string) {
	fmt.Fprintf(p.w, "%s✓%s %s\n", green, reset, msg)
}

// Warn prints a warning line.
func (p *Printer) Warn(msg string) {
	fmt.Fprintf(p.w, "%s!%s %s\n", yellow, reset, msg)
}

// Error prints an error line.
func (p *Printer) Error(msg string) {
	fmt.Fprintf(p.w, "%s✗%s %s\n", red, reset, msg)
}

// Info prints a dim informational line.
func (p *Printer) Info(msg string) {
	fmt.Fprintf(p.w, "%s%s%s\n", gray, reset, msg)
}

// Detail prints a label: value pair.
func (p *Printer) Detail(label, value string) {
	fmt.Fprintf(p.w, "  %s%s%s: %s\n", cyan, label, reset, value)
}

// Header prints a section header.
func (p *Printer) Header(msg string) {
	fmt.Fprintf(p.w, "\n%s== %s ==%s\n", bold, msg, reset)
}

func colorFor(icon string) string {
	switch icon {
	case "✓":
		return green
	case "!":
		return yellow
	case "✗":
		return red
	default:
		return cyan
	}
}
