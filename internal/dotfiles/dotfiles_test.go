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

func TestRenderExportsRenderedTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NESTOR_RENDER_TEST", "rendered-value")
	src := filepath.Join(dir, "editor.tmpl")
	if err := os.WriteFile(src, []byte(`ED={{env "NESTOR_RENDER_TEST"}}`), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	got, err := Render(src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(got) != "ED=rendered-value" {
		t.Fatalf("rendered output = %q, want %q", got, "ED=rendered-value")
	}
}

func TestRenderParseError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "broken.tmpl")
	if err := os.WriteFile(src, []byte(`{{ if }}`), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if _, err := Render(src); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestSymlinkFallbackCopyOnFailure(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "plain.conf")
	if err := os.WriteFile(src, []byte("plain content\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	// Symlinking onto a non-empty directory path fails, forcing the
	// fallbackCopy branch.
	blocked := filepath.Join(dir, "blocked")
	if err := os.MkdirAll(filepath.Join(blocked, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	d := Deployer{Strategy: StrategySymlink, Source: dir}
	r := d.Deploy(Template{Src: "plain.conf", Dest: blocked})

	if r.Status != StatusError {
		t.Fatalf("expected Error when symlink and fallback both fail, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", r.Err)
	}
}

func TestFallbackCopySuccess(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src-file")
	if err := os.WriteFile(src, []byte("fallback payload"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "dest-file")

	if err := fallbackCopy(src, dest); err != nil {
		t.Fatalf("fallbackCopy: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != "fallback payload" {
		t.Fatalf("content = %q", got)
	}
}

func TestFallbackCopyMissingSrc(t *testing.T) {
	dir := t.TempDir()
	if err := fallbackCopy(filepath.Join(dir, "nope"), filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected error for missing src, got nil")
	}
}

func TestFallbackCopyDestIsDirFails(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src-file")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	// os.Create on a directory path fails regardless of user privileges.
	destDir := filepath.Join(dir, "dest-dir")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := fallbackCopy(src, destDir); err == nil {
		t.Fatal("expected create error when dest is a directory, got nil")
	}
}

func TestStatusString(t *testing.T) {
	cases := map[Status]string{
		StatusDeployed: "deployed",
		StatusSkipped:  "skipped",
		StatusError:    "error",
		Status(99):     "unknown",
	}
	for status, want := range cases {
		if got := status.String(); got != want {
			t.Errorf("Status(%d).String() = %q, want %q", int(status), got, want)
		}
	}
}

func TestCheckStatusString(t *testing.T) {
	cases := map[CheckStatus]string{
		CheckPresent:    "present",
		CheckDrifted:    "drifted",
		CheckAbsent:     "absent",
		CheckSrcMissing: "src-missing",
		CheckStatus(99): "unknown",
	}
	for status, want := range cases {
		if got := status.String(); got != want {
			t.Errorf("CheckStatus(%d).String() = %q, want %q", int(status), got, want)
		}
	}
}

func TestSamePath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.conf")

	if !samePath(a, a) {
		t.Error("identical paths should be same")
	}
	rel := "a.conf"
	if !samePath(a, rel) {
		t.Skipf("relative comparison skipped (cwd differs): %q vs %q", a, rel)
	}
	if samePath(filepath.Join(dir, "a.conf"), filepath.Join(dir, "b.conf")) {
		t.Error("different paths should not be same")
	}
}

func TestSamePathEmptyStrings(t *testing.T) {
	if !samePath("", "") {
		t.Error("two empty paths should compare equal via fallback")
	}
}

func TestCheckSymlinkPresent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "linked.conf")
	if err := os.WriteFile(src, []byte("symlinked\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "out", ".linked.conf")

	d := Deployer{Strategy: StrategySymlink, Source: dir}
	if r := d.Deploy(Template{Src: "linked.conf", Dest: dest}); r.Status != StatusDeployed {
		t.Fatalf("deploy: %s (%v)", r.Status, r.Err)
	}

	if status := d.Check("linked.conf", dest); status != CheckPresent {
		t.Fatalf("expected Present, got %s", status)
	}
}

func TestCheckSymlinkDriftedToWrongTarget(t *testing.T) {
	dir := t.TempDir()
	srcA := filepath.Join(dir, "a.conf")
	srcB := filepath.Join(dir, "b.conf")
	os.WriteFile(srcA, []byte("a\n"), 0o600)
	os.WriteFile(srcB, []byte("b\n"), 0o600)

	dest := filepath.Join(dir, "out", ".conf")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(srcB, dest); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	d := Deployer{Strategy: StrategySymlink, Source: dir}
	if status := d.Check("a.conf", dest); status != CheckDrifted {
		t.Fatalf("expected Drifted for wrong-target symlink, got %s", status)
	}
}

func TestCheckTemplateSrcUnreadableReportsDrifted(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.tmpl")
	if err := os.WriteFile(src, []byte("{{ if }}"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	// Dest must exist so Check gets past the Lstat-absent branch and into
	// the render-compare, where the broken template forces CheckDrifted.
	dest := filepath.Join(dir, "out", "bad")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(dest, []byte("stale content"), 0o600); err != nil {
		t.Fatalf("write dest: %v", err)
	}
	d := Deployer{Strategy: StrategyCopy, Source: dir}
	if status := d.Check("bad.tmpl", dest); status != CheckDrifted {
		t.Fatalf("expected Drifted on unrenderable template src, got %s", status)
	}
}

func TestDeployCreatesNestedDestDirs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "nested.conf")
	if err := os.WriteFile(src, []byte("deep\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "a", "b", "c", ".nested")

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "nested.conf", Dest: dest})
	if r.Status != StatusDeployed {
		t.Fatalf("expected Deployed, got %s (%v)", r.Status, r.Err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("dest not created: %v", err)
	}
}

func TestDeployAbsoluteSrcOverridesSource(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	// File lives in dirB; Deployer.Source points at dirA. An absolute Src
	// must win over Source.
	file := filepath.Join(dirB, "abs.conf")
	if err := os.WriteFile(file, []byte("from-b\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	d := Deployer{Strategy: StrategyCopy, Source: dirA}
	r := d.Deploy(Template{Src: file, Dest: filepath.Join(dirA, "out", ".abs")})
	if r.Status != StatusDeployed {
		t.Fatalf("expected Deployed, got %s (%v)", r.Status, r.Err)
	}
	got, err := os.ReadFile(filepath.Join(dirA, "out", ".abs"))
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != "from-b\n" {
		t.Fatalf("content = %q, want %q", got, "from-b\n")
	}
}

func TestDeployUnrenderableTemplateReturnsReadError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "broken.tmpl")
	if err := os.WriteFile(src, []byte("{{ if }}"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "broken.tmpl", Dest: filepath.Join(dir, "out", ".broken")})
	if r.Status != StatusError {
		t.Fatalf("expected Error, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "read") {
		t.Fatalf("expected read-prefixed error, got %v", r.Err)
	}
}

func TestDeployDestIsDirReturnsWriteError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "w.conf")
	if err := os.WriteFile(src, []byte("w\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	// Writing onto a directory path fails with EISDIR regardless of user.
	destDir := filepath.Join(dir, "dest-is-dir")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "w.conf", Dest: destDir})
	if r.Status != StatusError {
		t.Fatalf("expected Error, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "write") {
		t.Fatalf("expected write-prefixed error, got %v", r.Err)
	}
}

func TestCopyMkdirFailure(t *testing.T) {
	// A dest whose ancestor is a file (not a dir) makes MkdirAll fail.
	dir := t.TempDir()
	src := filepath.Join(dir, "m.conf")
	if err := os.WriteFile(src, []byte("m\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "m.conf", Dest: filepath.Join(blocker, "nested", ".m")})
	if r.Status != StatusError {
		t.Fatalf("expected Error, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "mkdir") {
		t.Fatalf("expected mkdir-prefixed error, got %v", r.Err)
	}
}

func TestDeployTemplateUndefinedKeyFailsNotNoValue(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "gitconfig.tmpl")
	// {{.GH_TOKEN}} has no data source today; the old behavior rendered the
	// literal "<no value>" into the deployed file instead of failing.
	if err := os.WriteFile(src, []byte("token = {{.GH_TOKEN}}\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "out", ".gitconfig")

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "gitconfig.tmpl", Dest: dest})

	if r.Status != StatusError {
		t.Fatalf("expected StatusError for undefined key, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "no entry for key") {
		t.Fatalf("expected missing-key error, got %v", r.Err)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Fatal("dest should not exist when rendering fails")
	}
}

func TestDeployTemplateEnvFuncUnaffectedByMissingKeyOption(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "shellrc.tmpl")
	t.Setenv("NESTOR_MK_TEST", "fine")
	if err := os.WriteFile(src, []byte(`export X={{env "NESTOR_MK_TEST"}} plain={{.}}, end`), 0o600); err != nil {
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
	if string(got) != "export X=fine plain=<no value>, end" {
		t.Fatalf("content mismatch: %q", got)
	}
}

func TestCheckTemplateUndefinedKeyReportsDrifted(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "kitty.conf.tmpl")
	if err := os.WriteFile(src, []byte("font={{.FONT}}\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "out", "kitty.conf")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("mkdir dest dir: %v", err)
	}
	if err := os.WriteFile(dest, []byte("font=monospace\n"), 0o600); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	if status := d.Check("kitty.conf.tmpl", dest); status != CheckDrifted {
		t.Fatalf("expected Drifted when render fails, got %s", status)
	}
}

func TestRenderUndefinedKeyErrors(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "editor.tmpl")
	if err := os.WriteFile(src, []byte("v={{.MISSING}}\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	out, err := Render(src)
	if err == nil {
		t.Fatalf("expected error, got output %q", out)
	}
	if !strings.Contains(err.Error(), "no entry for key") {
		t.Fatalf("expected missing-key error, got %v", err)
	}
}
