package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestVersionOutput(t *testing.T) {
	// Set known build values
	oldV, oldC, oldD := buildVersion, buildCommit, buildDate
	buildVersion, buildCommit, buildDate = "v0.1.0", "abcdef0", "2026-06-23T15:00:00Z"
	defer func() { buildVersion, buildCommit, buildDate = oldV, oldC, oldD }()

	var out bytes.Buffer
	v := &cobra.Command{Use: "version", Run: versionCmd.Run}
	v.SetOut(&out)
	v.Run(v, nil)

	got := out.String()
	for _, want := range []string{"v0.1.0", "abcdef0", "2026-06-23T15:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("version output missing %q\noutput:\n%s", want, got)
		}
	}
}

func TestSetVersion(t *testing.T) {
	SetVersion("v9.9.9", "deadbeef", "2099-01-01")
	if buildVersion != "v9.9.9" {
		t.Errorf("buildVersion = %q, want v9.9.9", buildVersion)
	}
	if buildCommit != "deadbeef" {
		t.Errorf("buildCommit = %q, want deadbeef", buildCommit)
	}
	if buildDate != "2099-01-01" {
		t.Errorf("buildDate = %q, want 2099-01-01", buildDate)
	}
}
