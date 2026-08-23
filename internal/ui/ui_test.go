package ui

import (
	"bytes"
	"strings"
	"testing"
)

// capture runs fn with a Printer writing to a buffer and returns the output.
func capture(fn func(*Printer)) string {
	var buf bytes.Buffer
	fn(New(&buf))
	return buf.String()
}

func TestStep(t *testing.T) {
	for _, tc := range []struct {
		icon string
		msg  string
		want string
	}{
		{"✓", "all good", green + "✓" + reset + " nestor: all good\n"},
		{"!", "hmm", yellow + "!" + reset + " nestor: hmm\n"},
		{"✗", "broke", red + "✗" + reset + " nestor: broke\n"},
		{"→", "other icon gets cyan", cyan + "→" + reset + " nestor: other icon gets cyan\n"},
	} {
		got := capture(func(p *Printer) { p.Step(tc.icon, tc.msg) })
		if got != tc.want {
			t.Errorf("Step(%q, %q) = %q, want %q", tc.icon, tc.msg, got, tc.want)
		}
	}
}

func TestOKWarnErrorInfo(t *testing.T) {
	if got, want := capture(func(p *Printer) { p.OK("installed") }), green+"✓"+reset+" installed\n"; got != want {
		t.Errorf("OK = %q, want %q", got, want)
	}
	if got, want := capture(func(p *Printer) { p.Warn("drifted") }), yellow+"!"+reset+" drifted\n"; got != want {
		t.Errorf("Warn = %q, want %q", got, want)
	}
	if got, want := capture(func(p *Printer) { p.Error("failed") }), red+"✗"+reset+" failed\n"; got != want {
		t.Errorf("Error = %q, want %q", got, want)
	}
	if got, want := capture(func(p *Printer) { p.Info("dim note") }), gray+"dim note"+reset+"\n"; got != want {
		t.Errorf("Info = %q, want %q", got, want)
	}
}

func TestDetail(t *testing.T) {
	got := capture(func(p *Printer) { p.Detail("OS", "linux") })
	want := "  " + cyan + "OS" + reset + ": linux\n"
	if got != want {
		t.Errorf("Detail = %q, want %q", got, want)
	}
}

func TestHeader(t *testing.T) {
	got := capture(func(p *Printer) { p.Header("Packages") })
	want := "\n" + bold + "== Packages ==" + reset + "\n"
	if got != want {
		t.Errorf("Header = %q, want %q", got, want)
	}
}

func TestPrinterWritesSequentially(t *testing.T) {
	got := capture(func(p *Printer) {
		p.Header("Section")
		p.Step("✓", "step one")
		p.Detail("label", "value")
		p.Info("done")
	})
	for _, want := range []string{
		"== Section ==",
		"nestor: step one",
		"label",
		"done",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("combined output missing %q; got %q", want, got)
		}
	}
	if !strings.HasSuffix(got, gray+"done"+reset+"\n") {
		t.Errorf("combined output should end with dimmed Info line; got %q", got)
	}
}
