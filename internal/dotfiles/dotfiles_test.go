package dotfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyStrategy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(src, []byte("[user]\n\tname = marcus\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "out", ".gitconfig")

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "gitconfig", Dest: dest})

	if r.Status != StatusDeployed {
		t.Fatalf("expected Deployed, got %s (%v)", r.Status, r.Err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != "[user]\n\tname = marcus\n" {
		t.Fatalf("content mismatch: %q", got)
	}
}

func TestCopyTemplateRendering(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "shellrc.tmpl")
	t.Setenv("NESTOR_TEST_USER", "testuser")
	if err := os.WriteFile(src, []byte(`export USER={{env "NESTOR_TEST_USER"}}`), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "out", ".shellrc")

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "shellrc.tmpl", Dest: dest})

	if r.Status != StatusDeployed {
		t.Fatalf("expected Deployed, got %s (%v)", r.Status, r.Err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != "export USER=testuser" {
		t.Fatalf("rendered content mismatch: %q", got)
	}
}

func TestSymlinkStrategy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tmux.conf")
	if err := os.WriteFile(src, []byte("set -g mouse on\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "out", ".tmux.conf")

	d := Deployer{Strategy: StrategySymlink, Source: dir}
	r := d.Deploy(Template{Src: "tmux.conf", Dest: dest})

	if r.Status != StatusDeployed {
		t.Fatalf("expected Deployed, got %s (%v)", r.Status, r.Err)
	}
	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if !strings.HasSuffix(target, "tmux.conf") {
		t.Fatalf("symlink target = %q, want suffix tmux.conf", target)
	}
}

func TestMissingSrc(t *testing.T) {
	dir := t.TempDir()
	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "nope", Dest: filepath.Join(dir, ".nope")})
	if r.Status != StatusError {
		t.Fatalf("expected Error, got %s", r.Status)
	}
}

func TestDeployAll(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	temps := []Template{
		{Src: "a", Dest: filepath.Join(dir, "out", ".a")},
		{Src: "b", Dest: filepath.Join(dir, "out", ".b")},
		{Src: "missing", Dest: filepath.Join(dir, "out", ".c")},
	}
	d := Deployer{Strategy: StrategyCopy, Source: dir}
	results := d.DeployAll(temps)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Status != StatusDeployed {
		t.Errorf("first should be Deployed, got %s", results[0].Status)
	}
	if results[2].Status != StatusError {
		t.Errorf("third should be Error, got %s", results[2].Status)
	}
}

func TestCheckPresent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "gitconfig")
	os.WriteFile(src, []byte("[user]\n\tname = marcus\n"), 0o600)

	dest := filepath.Join(dir, "out", ".gitconfig")
	d := Deployer{Strategy: StrategyCopy, Source: dir}
	d.Deploy(Template{Src: "gitconfig", Dest: dest})

	if status := d.Check("gitconfig", dest); status != CheckPresent {
		t.Fatalf("expected Present, got %s", status)
	}
}

func TestCheckDrifted(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "vimrc")
	os.WriteFile(src, []byte("set nu\n"), 0o600)

	dest := filepath.Join(dir, "out", ".vimrc")
	d := Deployer{Strategy: StrategyCopy, Source: dir}
	d.Deploy(Template{Src: "vimrc", Dest: dest})

	// Simulate user editing the dest file
	os.WriteFile(dest, []byte("set nocompatible\n"), 0o600)

	if status := d.Check("vimrc", dest); status != CheckDrifted {
		t.Fatalf("expected Drifted, got %s", status)
	}
}

func TestCheckAbsent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bashrc")
	os.WriteFile(src, []byte("alias ll='ls -la'\n"), 0o600)

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	status := d.Check("bashrc", filepath.Join(dir, "nowhere", ".bashrc"))
	if status != CheckAbsent {
		t.Fatalf("expected Absent, got %s", status)
	}
}

func TestCheckSrcMissing(t *testing.T) {
	dir := t.TempDir()
	d := Deployer{Strategy: StrategyCopy, Source: dir}
	status := d.Check("nonexistent", filepath.Join(dir, ".nonexistent"))
	if status != CheckSrcMissing {
		t.Fatalf("expected SrcMissing, got %s", status)
	}
}

func TestCheckTemplateDrifted(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "config.tmpl")
	t.Setenv("NESTOR_TEST_NAME", "marcus")
	os.WriteFile(src, []byte("name = {{env \"NESTOR_TEST_NAME\"}}\n"), 0o600)

	dest := filepath.Join(dir, "out", "config")
	d := Deployer{Strategy: StrategyCopy, Source: dir}
	d.Deploy(Template{Src: "config.tmpl", Dest: dest})

	// Edit dest—  should drift
	os.WriteFile(dest, []byte("name = someone_else\n"), 0o600)

	if status := d.Check("config.tmpl", dest); status != CheckDrifted {
		t.Fatalf("expected Drifted, got %s", status)
	}
}
