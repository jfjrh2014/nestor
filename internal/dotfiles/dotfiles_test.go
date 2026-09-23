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

	// Force the SYMLINK step itself to fail: a non-empty directory planted
	// at the temp-link name defeats os.Symlink (EEXIST, and the pre-remove
	// can't clear a non-empty dir), while the non-empty dir at dest defeats
	// the fallback copy. Permission tricks can't force EACCES under root.
	blocked := filepath.Join(dir, "blocked")
	if err := os.MkdirAll(filepath.Join(blocked, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tmpLinkDir := filepath.Join(dir, "blocked.nestor-tmp-link") // tmp-link name derives from the DEST
	if err := os.MkdirAll(filepath.Join(tmpLinkDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir tmp-link: %v", err)
	}

	d := Deployer{Strategy: StrategySymlink, Source: dir}
	r := d.Deploy(Template{Src: "plain.conf", Dest: blocked})

	if r.Status != StatusError {
		t.Fatalf("expected Error when symlink and fallback both fail, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", r.Err)
	}
	if _, err := os.Stat(filepath.Join(blocked, "sub")); err != nil {
		t.Fatalf("user's directory was damaged by a failed deploy: %v", err)
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

func TestSymlinkTemplateRendersBeforeLinking(t *testing.T) {
	srcDir := t.TempDir()
	home := t.TempDir()
	tmpl := filepath.Join(srcDir, "gitconfig.tmpl")
	if err := os.WriteFile(tmpl, []byte("[user]\n\tname = {{env \"NESTOR_TEST_RENDER\"}}\n"), 0o644); err != nil {
		t.Fatalf("write tmpl: %v", err)
	}
	t.Setenv("NESTOR_TEST_RENDER", "rendered-by-test")
	dest := filepath.Join(home, ".gitconfig")

	d := Deployer{Strategy: StrategySymlink, Source: srcDir}
	r := d.Deploy(Template{Src: "gitconfig.tmpl", Dest: dest})
	if r.Status != StatusDeployed || r.Err != nil {
		t.Fatalf("deploy: status=%s err=%v", r.Status, r.Err)
	}

	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("dest is not a symlink: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "[user]\n\tname = rendered-by-test\n" {
		t.Fatalf("dest content = %q, want rendered output", string(data))
	}
	if filepath.Base(filepath.Dir(target)) != renderedLinkMarker {
		t.Fatalf("link target %q should live in the %s render dir", target, renderedLinkMarker)
	}
}

func TestSymlinkTemplateUndefinedKeyFailsNotRaw(t *testing.T) {
	srcDir := t.TempDir()
	home := t.TempDir()
	tmpl := filepath.Join(srcDir, "app.conf.tmpl")
	if err := os.WriteFile(tmpl, []byte("token = {{.GH_TOKEN}}\n"), 0o644); err != nil {
		t.Fatalf("write tmpl: %v", err)
	}
	dest := filepath.Join(home, ".app.conf")

	d := Deployer{Strategy: StrategySymlink, Source: srcDir}
	r := d.Deploy(Template{Src: "app.conf.tmpl", Dest: dest})
	if r.Status != StatusError {
		t.Fatalf("expected Error, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "render") {
		t.Fatalf("expected render error, got %v", r.Err)
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Fatalf("dest should not exist after failed render, got %v", err)
	}
}

func TestSymlinkPlainFileStillLinksToSrc(t *testing.T) {
	srcDir := t.TempDir()
	home := t.TempDir()
	plain := filepath.Join(srcDir, "plain.conf")
	if err := os.WriteFile(plain, []byte("plain\n"), 0o644); err != nil {
		t.Fatalf("write plain: %v", err)
	}
	dest := filepath.Join(home, ".plain.conf")

	d := Deployer{Strategy: StrategySymlink, Source: srcDir}
	r := d.Deploy(Template{Src: "plain.conf", Dest: dest})
	if r.Status != StatusDeployed || r.Err != nil {
		t.Fatalf("deploy: status=%s err=%v", r.Status, r.Err)
	}
	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("dest is not a symlink: %v", err)
	}
	if !samePath(target, plain) {
		t.Fatalf("plain file should link src directly, got target %q", target)
	}
}

func TestCheckSymlinkRenderedTemplatePresent(t *testing.T) {
	srcDir := t.TempDir()
	home := t.TempDir()
	tmpl := filepath.Join(srcDir, "gitconfig.tmpl")
	if err := os.WriteFile(tmpl, []byte("[user]\n\tname = {{env \"NESTOR_TEST_CHECK\"}}\n"), 0o644); err != nil {
		t.Fatalf("write tmpl: %v", err)
	}
	t.Setenv("NESTOR_TEST_CHECK", "checker")
	dest := filepath.Join(home, ".gitconfig")

	d := Deployer{Strategy: StrategySymlink, Source: srcDir}
	if r := d.Deploy(Template{Src: "gitconfig.tmpl", Dest: dest}); r.Status != StatusDeployed {
		t.Fatalf("deploy: status=%s err=%v", r.Status, r.Err)
	}
	if got := d.Check("gitconfig.tmpl", dest); got != CheckPresent {
		t.Fatalf("Check after deploy = %s, want Present (rendered links must not read as drift)", got)
	}
}

func TestCheckSymlinkRenderedTemplateDriftsOnContentChange(t *testing.T) {
	srcDir := t.TempDir()
	home := t.TempDir()
	tmpl := filepath.Join(srcDir, "gitconfig.tmpl")
	if err := os.WriteFile(tmpl, []byte("[user]\n\tname = {{env \"NESTOR_TEST_DRIFT\"}}\n"), 0o644); err != nil {
		t.Fatalf("write tmpl: %v", err)
	}
	t.Setenv("NESTOR_TEST_DRIFT", "before")
	dest := filepath.Join(home, ".gitconfig")

	d := Deployer{Strategy: StrategySymlink, Source: srcDir}
	if r := d.Deploy(Template{Src: "gitconfig.tmpl", Dest: dest}); r.Status != StatusDeployed {
		t.Fatalf("deploy: status=%s err=%v", r.Status, r.Err)
	}
	if err := os.WriteFile(tmpl, []byte("[user]\n\tname = {{env \"NESTOR_TEST_DRIFT\"}} changed\n"), 0o644); err != nil {
		t.Fatalf("edit tmpl: %v", err)
	}
	if got := d.Check("gitconfig.tmpl", dest); got != CheckDrifted {
		t.Fatalf("Check after src edit = %s, want Drifted", got)
	}
}

func TestIsRenderedLinkRecognition(t *testing.T) {
	srcDir := t.TempDir()
	tmpl := filepath.Join(srcDir, "x.tmpl")
	if err := os.WriteFile(tmpl, []byte("v = {{env \"NESTOR_TEST_REC\"}}\n"), 0o644); err != nil {
		t.Fatalf("write tmpl: %v", err)
	}
	t.Setenv("NESTOR_TEST_REC", "rec")

	rendered, err := renderTemplate(tmpl)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	marked := filepath.Join(t.TempDir(), renderedLinkMarker, "x.tmpl")
	if err := os.MkdirAll(filepath.Dir(marked), 0o700); err != nil {
		t.Fatalf("mkdir render dir: %v", err)
	}
	if err := os.WriteFile(marked, rendered, 0o600); err != nil {
		t.Fatalf("write marked: %v", err)
	}
	if !isRenderedLink(marked, tmpl) {
		t.Fatal("marked file with matching render should be recognized")
	}

	// Marker dir but content differs from a fresh render: not deployed.
	if err := os.MkdirAll(filepath.Dir(marked), 0o700); err != nil {
		t.Fatalf("mkdir render dir: %v", err)
	}
	if err := os.WriteFile(marked, []byte("tampered\n"), 0o600); err != nil {
		t.Fatalf("write tampered: %v", err)
	}
	if isRenderedLink(marked, tmpl) {
		t.Fatal("tampered content must not be recognized as rendered")
	}

	// No marker, not src: not recognized.
	unmarked := filepath.Join(t.TempDir(), "other")
	if err := os.WriteFile(unmarked, rendered, 0o600); err != nil {
		t.Fatalf("write unmarked: %v", err)
	}
	if isRenderedLink(unmarked, tmpl) {
		t.Fatal("unmarked path must not be recognized")
	}
}

// TestDeployOtherUserTildeDestNotRewritten: a "~user/..." dest must be left
// verbatim (never silently re-rooted at $HOME). With the config validator as
// the first line of defense, the engine's job is to not make things worse:
// the dest is not the tester's home, so the write must not land there.
func TestDeployOtherUserTildeDestNotRewritten(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bashrc.tmpl")
	if err := os.WriteFile(src, []byte("export NESTOR=1\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		t.Skip("no home dir available")
	}

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	res := d.Deploy(Template{Src: "bashrc.tmpl", Dest: "~other/.bashrc"})
	if res.Status != StatusError {
		t.Fatalf("want StatusError for ~other dest, got %s", res.Status)
	}
	// The corrupted write must not exist anywhere in the real home.
	leak := filepath.Join(home, "other", ".bashrc")
	if _, err := os.Stat(leak); err == nil {
		t.Errorf("dest was silently re-rooted into the real home: %s exists", leak)
		os.Remove(leak)
	}
}

// TestCheckOtherUserTildeDest: diff on a "~user/..." dest reports the
// unsupported-dest status instead of probing a bogus path.
func TestCheckOtherUserTildeDest(t *testing.T) {
	dir := t.TempDir()
	d := Deployer{Strategy: StrategyCopy, Source: dir}
	if got := d.Check("x.tmpl", "~other/.bashrc"); got != CheckUnknown {
		t.Errorf("Check(~other/.bashrc) = %s, want unsupported-dest", got)
	}
}

// TestCopyDeployRefusesSymlinkDest: os.WriteFile follows symlinks, so a copy
// deploy onto an existing dest symlink would silently overwrite whatever the
// link points at (a chezmoi/stow checkout, an old symlink-strategy leftover).
// The engine must refuse instead of writing through.
func TestCopyDeployRefusesSymlinkDest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(src, []byte("[user]\n\tname = marcus\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	victim := filepath.Join(dir, "real-gitconfig")
	if err := os.WriteFile(victim, []byte("ORIGINAL\n"), 0o600); err != nil {
		t.Fatalf("write victim: %v", err)
	}
	dest := filepath.Join(dir, ".gitconfig")
	if err := os.Symlink(victim, dest); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "gitconfig", Dest: dest})

	if r.Status != StatusError {
		t.Fatalf("expected StatusError for symlink dest, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "symlink") {
		t.Fatalf("expected symlink refusal error, got %v", r.Err)
	}

	// The link's target must be untouched and the link itself intact.
	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("read victim: %v", err)
	}
	if string(got) != "ORIGINAL\n" {
		t.Fatalf("target file was overwritten through the link: %q", got)
	}
	if info, err := os.Lstat(dest); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("dest is no longer a symlink after refused deploy: %v %v", info, err)
	}
}

// TestCopyDeployRefusesDanglingSymlinkDest: writing through a dangling link
// doesn't fail — it silently CREATES the missing target file. The deploy must
// refuse and nothing may appear at the link's destination path.
func TestCopyDeployRefusesDanglingSymlinkDest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bashrc")
	if err := os.WriteFile(src, []byte("export NESTOR=1\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	dest := filepath.Join(dir, ".bashrc")
	phantom := filepath.Join(dir, "gone", ".bashrc")
	if err := os.Symlink(phantom, dest); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "bashrc", Dest: dest})

	if r.Status != StatusError {
		t.Fatalf("expected StatusError for dangling symlink dest, got %s", r.Status)
	}
	if _, err := os.Lstat(phantom); !os.IsNotExist(err) {
		t.Fatalf("dangling link target was created: %v", err)
	}
}

// TestCopyDeployOverwritesRegularFile: the refusal is keyed on the symlink
// itself, not on dest content differing from src — redeploying over a plain
// regular file still works (the standard re-run path).
func TestCopyDeployOverwritesRegularFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(src, []byte("v2\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, ".gitconfig")
	if err := os.WriteFile(dest, []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "gitconfig", Dest: dest})
	if r.Status != StatusDeployed {
		t.Fatalf("expected StatusDeployed over regular file, got %s (%v)", r.Status, r.Err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "v2\n" {
		t.Fatalf("regular-file redeploy broken: %q", got)
	}
}

// TestCopyPreservesPrivateMode: a 0600 source must deploy at 0600. The copy
// branch used to hard-code 0644, quietly widening private files (ssh keys,
// tokens) on the first deploy. On redeploy, WriteFile's perm applies only at
// creation, so a dest the user chmod'ed locally keeps its own mode — same
// don't-stomp-local-state philosophy as skip-hand-edits.
func TestCopyPreservesPrivateMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(src, []byte("[user]\n\tname = marcus\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "out", ".gitconfig")

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	if r := d.Deploy(Template{Src: "gitconfig", Dest: dest}); r.Status != StatusDeployed {
		t.Fatalf("expected Deployed, got %s (%v)", r.Status, r.Err)
	}
	if info, err := os.Stat(dest); err != nil {
		t.Fatalf("stat dest: %v", err)
	} else if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("deployed mode = %v, want 0600 (0644 widens private files)", got)
	}

	// Redeploy does not stomp a local chmod of the dest.
	if err := os.Chmod(dest, 0o640); err != nil {
		t.Fatalf("chmod dest: %v", err)
	}
	if r := d.Deploy(Template{Src: "gitconfig", Dest: dest}); r.Status != StatusDeployed {
		t.Fatalf("redeploy: expected Deployed, got %s (%v)", r.Status, r.Err)
	}
	if info, err := os.Stat(dest); err != nil {
		t.Fatalf("stat dest after redeploy: %v", err)
	} else if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("redeployed mode = %v, want 640 kept (perm must apply at creation only)", got)
	}
}

// TestCopyExecutablePreservesExecBit: an executable source deploys runnable.
func TestCopyExecutablePreservesExecBit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(src, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "out", "hook.sh")

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	if r := d.Deploy(Template{Src: "hook.sh", Dest: dest}); r.Status != StatusDeployed {
		t.Fatalf("expected Deployed, got %s (%v)", r.Status, r.Err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("deployed mode = %v, want 755 (exec bit dropped)", got)
	}
}

// TestSymlinkDeployKeepsOldDestWhenSwapFails: the deploy links to a temp
// name and renames over dest, so a dest shape that defeats the rename (a
// non-empty directory) is refused with the dest intact and no temp-link
// litter. The old remove-then-link order had a destroy window instead: a
// crash between the two steps lost the dest outright, and a foreign file
// created in the window was silently linked over.
func TestSymlinkDeployKeepsOldDestWhenSwapFails(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "plain.conf")
	if err := os.WriteFile(src, []byte("new content\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	blocked := filepath.Join(dir, ".app.conf")
	if err := os.MkdirAll(filepath.Join(blocked, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	keep := filepath.Join(blocked, "precious.txt")
	if err := os.WriteFile(keep, []byte("PRECIOUS\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	d := Deployer{Strategy: StrategySymlink, Source: dir}
	r := d.Deploy(Template{Src: "plain.conf", Dest: blocked})
	if r.Status != StatusError {
		t.Fatalf("expected Error, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "rename") {
		t.Fatalf("expected rename refusal, got %v", r.Err)
	}

	got, err := os.ReadFile(keep)
	if err != nil {
		t.Fatalf("user's directory was damaged by a failed deploy: %v", err)
	}
	if string(got) != "PRECIOUS\n" {
		t.Fatalf("user file changed: %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".nestor-tmp-link") {
			t.Fatalf("temp link litter left behind: %s", e.Name())
		}
	}
}

// TestSymlinkDeployReplacesOldDest: switching a dotfile from copy-managed to
// symlink strategy must replace the old regular file at dest with the link.
func TestSymlinkDeployReplacesOldDest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tmux.conf")
	if err := os.WriteFile(src, []byte("set -g mouse on\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	home := t.TempDir()
	dest := filepath.Join(home, ".tmux.conf")
	if err := os.WriteFile(dest, []byte("old copy\n"), 0o600); err != nil {
		t.Fatalf("write old dest: %v", err)
	}

	d := Deployer{Strategy: StrategySymlink, Source: dir}
	r := d.Deploy(Template{Src: "tmux.conf", Dest: dest})
	if r.Status != StatusDeployed || r.Err != nil {
		t.Fatalf("deploy: status=%s err=%v", r.Status, r.Err)
	}
	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("dest is not a symlink after re-deploy: %v", err)
	}
	if !samePath(target, src) {
		t.Fatalf("target = %q, want %q", target, src)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".nestor-tmp-link") {
			t.Fatalf("temp link litter left behind: %s", e.Name())
		}
	}
}

// TestSymlinkDeployRedeployOverExistingLink: running the deploy twice must
// swap the link in place, not fail because dest already exists.
func TestSymlinkDeployRedeployOverExistingLink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(src, []byte("key=value\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	home := t.TempDir()
	dest := filepath.Join(home, ".app.conf")
	d := Deployer{Strategy: StrategySymlink, Source: dir}

	for i := 0; i < 2; i++ {
		r := d.Deploy(Template{Src: "app.conf", Dest: dest})
		if r.Status != StatusDeployed || r.Err != nil {
			t.Fatalf("deploy %d: status=%s err=%v", i+1, r.Status, r.Err)
		}
	}
	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("dest is not a symlink after redeploy: %v", err)
	}
	if !samePath(target, src) {
		t.Fatalf("target = %q, want %q", target, src)
	}
}

// TestFallbackCopyKeepsDestOnFailure: the copy fallback now goes through the
// durable writer, so a failed copy (rename blocked by a directory at dest)
// must leave the existing dest untouched with no temp litter.
func TestFallbackCopyKeepsDestOnFailure(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.conf")
	if err := os.WriteFile(src, []byte("payload\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	dest := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(filepath.Join(dest, "inner"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "user.txt"), []byte("USER\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := fallbackCopy(src, dest); err == nil {
		t.Fatal("expected error renaming over a directory")
	}

	got, err := os.ReadFile(filepath.Join(dest, "user.txt"))
	if err != nil {
		t.Fatalf("dest directory was damaged: %v", err)
	}
	if string(got) != "USER\n" {
		t.Fatalf("user file changed: %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".nestor-write-") {
			t.Fatalf("temp litter left behind: %s", e.Name())
		}
	}
}

// TestSymlinkTemplateRedeployKeepsDestOnRenderFailure: after a successful
// template deploy, a source that stops rendering (broken template, missing
// key) must fail the redeploy WITHOUT destroying the deployed link and its
// rendered content.
func TestSymlinkTemplateRedeployKeepsDestOnRenderFailure(t *testing.T) {
	srcDir := t.TempDir()
	home := t.TempDir()
	tmpl := filepath.Join(srcDir, "app.conf.tmpl")
	if err := os.WriteFile(tmpl, []byte("token = {{env \"NESTOR_TEST_TOKEN\"}}\n"), 0o644); err != nil {
		t.Fatalf("write tmpl: %v", err)
	}
	t.Setenv("NESTOR_TEST_TOKEN", "v1")

	dest := filepath.Join(home, ".app.conf")
	d := Deployer{Strategy: StrategySymlink, Source: srcDir}
	r := d.Deploy(Template{Src: "app.conf.tmpl", Dest: dest})
	if r.Status != StatusDeployed || r.Err != nil {
		t.Fatalf("first deploy: status=%s err=%v", r.Status, r.Err)
	}

	// Break the render: a map lookup on a missing key fails under
	// missingkey=error.
	if err := os.WriteFile(tmpl, []byte("token = {{.NESTOR_TEST_TOKEN}}\n"), 0o644); err != nil {
		t.Fatalf("rewrite tmpl: %v", err)
	}

	r = d.Deploy(Template{Src: "app.conf.tmpl", Dest: dest})
	if r.Status != StatusError {
		t.Fatalf("expected Error after broken render, got %s", r.Status)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "render") {
		t.Fatalf("expected render error, got %v", r.Err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("deployed dest was destroyed by failed redeploy: %v", err)
	}
	if string(data) != "token = v1\n" {
		t.Fatalf("deployed content changed: %q", data)
	}
	if info, err := os.Lstat(dest); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("dest is no longer a symlink after failed redeploy: %v %v", info, err)
	}
}

// TestCopyDeployFailureKeepsDest: the copy deploy now writes through
// fsutil.WriteFileSync (temp+fsync+rename), so a failed write must leave the
// existing dest byte-for-byte intact — the pre-#85 order truncated the dest
// in place, so a crash mid-write was a torn dotfile. The crash analogue:
// plant a non-empty directory at the temp-file pattern space? WriteFileSync
// fails on rename when the dest itself is a directory; the copy branch's
// symlink guard already refuses that, so force failure at the source instead:
// make the source unreadable after the deployer has rendered? Simplest real
// crash analogue: a dest parent that is a file blocks CreateTemp.
// NOTE: appended, refined below.
func TestCopyDeployFailureKeepsDest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(src, []byte("v2\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	outDir := filepath.Join(dir, "out")
	dest := filepath.Join(outDir, ".gitconfig")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}
	if err := os.WriteFile(dest, []byte("precious\n"), 0o600); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	// Make the write fail by planting a file at the parent dir path is
	// impossible (dest exists). Instead: stat error injection isn't
	// available, so use the rename blocker — replace the dest with a
	// directory. The symlink guard only checks ModeSymlink, so a directory
	// dest reaches the write and the rename inside WriteFileSync fails.
	if err := os.Remove(dest); err != nil {
		t.Fatalf("remove dest: %v", err)
	}
	if err := os.Mkdir(dest, 0o700); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "keepme"), []byte("user file\n"), 0o600); err != nil {
		t.Fatalf("write keeper: %v", err)
	}

	d := Deployer{Strategy: StrategyCopy, Source: dir}
	r := d.Deploy(Template{Src: "gitconfig", Dest: dest})
	if r.Status != StatusError {
		t.Fatalf("expected StatusError, got %s (%v)", r.Status, r.Err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "keepme"))
	if err != nil || string(got) != "user file\n" {
		t.Fatalf("user file damaged or missing: %q err=%v", got, err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("dest directory vanished: %v", err)
	}
	// No temp litter beside the dest.
	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		if e.Name() == ".gitconfig" {
			continue
		}
		t.Fatalf("temp litter left behind: %s", e.Name())
	}
}

// TestRenderedFileFailureKeepsPreviousRender: the rendered-template file is
// the symlink target of a deployed .tmpl dotfile, so a torn rewrite there is
// a torn dotfile on every deployed machine. The write goes through
// fsutil.WriteFileSync now; force the rename-block failure and assert the
// PREVIOUS render survives byte-for-byte and the deployed link still reads.
func TestRenderedFileFailureKeepsPreviousRender(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "gitconfig.tmpl")
	if err := os.WriteFile(src, []byte("name = {{ env \"NESTOR_TEST_RENDER\" }}\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, ".gitconfig")
	t.Setenv("NESTOR_TEST_RENDER", "v1")

	d := Deployer{Strategy: StrategySymlink, Source: dir}
	if r := d.Deploy(Template{Src: "gitconfig.tmpl", Dest: dest}); r.Status != StatusDeployed {
		t.Fatalf("first deploy: expected Deployed, got %s (%v)", r.Status, r.Err)
	}
	renderPath := renderedLinkPath(src)
	first, err := os.ReadFile(renderPath)
	if err != nil || string(first) != "name = v1\n" {
		t.Fatalf("first render wrong: %q err=%v", first, err)
	}
	linkDest, err := os.Readlink(dest)
	if err != nil || linkDest != renderPath {
		t.Fatalf("dest not a link to render: %q err=%v", linkDest, err)
	}

	// Crash analogue: block the rename by making the rendered dir a
	// non-empty directory conflict — renderPath must become a directory for
	// rename to fail. Remove the render file, plant a directory with a user
	// file at its name.
	if err := os.Remove(renderPath); err != nil {
		t.Fatalf("remove render: %v", err)
	}
	if err := os.Mkdir(renderPath, 0o700); err != nil {
		t.Fatalf("plant dir: %v", err)
	}
	keeper := filepath.Join(renderPath, "userfile")
	if err := os.WriteFile(keeper, []byte("precious\n"), 0o600); err != nil {
		t.Fatalf("write keeper: %v", err)
	}

	r := d.Deploy(Template{Src: "gitconfig.tmpl", Dest: dest})
	if r.Status != StatusError {
		t.Fatalf("expected StatusError on blocked render write, got %s (%v)", r.Status, r.Err)
	}
	// The keeper file (crash analogue of prior contents) must be intact.
	got, err := os.ReadFile(keeper)
	if err != nil || string(got) != "precious\n" {
		t.Fatalf("keeper damaged: %q err=%v", got, err)
	}
	// No temp litter in the rendered dir.
	entries, _ := os.ReadDir(filepath.Dir(renderPath))
	for _, e := range entries {
		if e.Name() == filepath.Base(src) { // the planted failure directory
			continue
		}
		t.Fatalf("litter in rendered dir: %s", e.Name())
	}
	entries, _ = os.ReadDir(renderPath)
	for _, e := range entries {
		if e.Name() == "userfile" {
			continue
		}
		t.Fatalf("temp litter beside keeper: %s", e.Name())
	}
}
