package cmd

import (
	"strings"
	"testing"
)

func TestResolveImporterUnknownSource(t *testing.T) {
	_, err := resolveImporter("gitwatch", "")
	if err == nil {
		t.Fatal("expected error for unknown source, got nil")
	}
	if !strings.Contains(err.Error(), "unknown import source") {
		t.Errorf("error = %q, want it to mention 'unknown import source'", err.Error())
	}
}

func TestResolveImporterPathWithAutoRejected(t *testing.T) {
	// auto-detect + a path is nonsensical: there's no single source to point at
	// without naming the tool. resolveImporter should reject it explicitly
	// rather than silently ignoring the path (the old dead-value failure mode).
	_, err := resolveImporter("", "/some/path")
	if err == nil {
		t.Fatal("expected error for auto + path, got nil")
	}
	if !strings.Contains(err.Error(), "explicit source") {
		t.Errorf("error = %q, want it to mention 'explicit source'", err.Error())
	}
}

func TestResolveImporterPathWithYadmRejected(t *testing.T) {
	// yadm reads `yadm list -a` and has no notion of a source path. A path
	// passed with yadm should be rejected, not silently dropped.
	_, err := resolveImporter("yadm", "/some/path")
	if err == nil {
		t.Fatal("expected error for yadm + path, got nil")
	}
	if !strings.Contains(err.Error(), "does not accept a path") {
		t.Errorf("error = %q, want it to mention 'does not accept a path'", err.Error())
	}
}

func TestResolveImporterBrewfileHonoursPath(t *testing.T) {
	// The seventh dead-value bug: an explicit Brewfile path used to be
	// declared, written into a global, and read with `_ = importSource`, but
	// never plumbed into NewBrewfile (which always got ""). Now the path
	// reaches the constructor. A missing file should surface NewBrewfile's
	// "not found" error — proving the path reached it, rather than the old
	// behaviour of silently scanning CWD.
	_, err := resolveImporter("brewfile", "/nonexistent/no-such-Brewfile")
	if err == nil {
		t.Fatal("expected error for missing Brewfile path, got nil (path was likely ignored)")
	}
	if !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "Brewfile not found") && !strings.Contains(err.Error(), "no file or directory") {
		t.Errorf("error = %q, want a not-found error proving the path reached NewBrewfile", err.Error())
	}
}

func TestResolveImporterChezmoiHonoursPath(t *testing.T) {
	// Same dead-value class: an explicit chezmoi source path must reach
	// NewChezmoi, not be discarded. A missing dir should surface its
	// "source dir not found" error.
	_, err := resolveImporter("chezmoi", "/nonexistent/no-such-chezmoi-dir")
	if err == nil {
		t.Fatal("expected error for missing chezmoi dir, got nil (path was likely ignored)")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want a not-found error proving the path reached NewChezmoi", err.Error())
	}
}
