package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
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

// --- runImport writer-injection tests (session #42) ---

// setupImportTest writes a nestor.yml in a temp dir, points cfgFile at it, and
// returns the dir (so callers can place a Brewfile or dotfile source inside).
func setupImportTest(t *testing.T) (dir, cfgPath string) {
	t.Helper()
	dir = t.TempDir()
	cfgPath = filepath.Join(dir, "nestor.yml")
	content := `version: 1
packages:
  common:
    - git
dotfiles:
  source: ` + dir + `/dotfiles
  strategy: copy
  templates: []
secrets:
  provider: env
  mappings: []
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "dotfiles"), 0755); err != nil {
		t.Fatal(err)
	}
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = "" })
	return dir, cfgPath
}

// writeImportBrewfile creates a Brewfile in dir and returns its path.
func writeImportBrewfile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "Brewfile")
	content := []byte(`brew "ripgrep"
brew "fd"
cask "visual-studio-code"
mas "Xcode"
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestImportBrewfileDryRunOutput asserts the preview path writes to the buffer,
// reports packages found + skipped, and does not write to config. Exercises the
// full fetch → display → dry-run-return path through the injected writer.
func TestImportBrewfileDryRunOutput(t *testing.T) {
	dir, cfgPath := setupImportTest(t)
	brewPath := writeImportBrewfile(t, dir)

	importDryRun = true
	t.Cleanup(func() { importDryRun = false })

	var buf bytes.Buffer
	if err := runImport("brewfile", brewPath, &buf); err != nil {
		t.Fatalf("runImport dry-run: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"source = brewfile",
		"packages found: 3",
		"+ brew: ripgrep",
		"+ brew/cask: visual-studio-code",
		"skipped: 1",
		"(dry-run, nothing written)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}

	// Config must not have changed on dry-run
	raw, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(raw), "ripgrep") {
		t.Error("dry-run should not write ripgrep into config")
	}
}

// TestImportBrewfileNothingNew asserts the "all items already in config" path
// writes to the buffer and returns nil without error.
func TestImportBrewfileNothingNew(t *testing.T) {
	dir, _ := setupImportTest(t)
	brewPath := writeImportBrewfile(t, dir)

	var buf bytes.Buffer
	importDryRun = false

	// First import adds the items
	if err := runImport("brewfile", brewPath, &buf); err != nil {
		t.Fatalf("first import: %v", err)
	}

	// Second import should find nothing new
	buf.Reset()
	if err := runImport("brewfile", brewPath, &buf); err != nil {
		t.Fatalf("second import: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"source = brewfile",
		"packages found: 3",
		"nothing new to add",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("second-import output missing %q\ngot:\n%s", want, out)
		}
	}
	if strings.Contains(out, "imported ") {
		t.Errorf("second import should not report items imported\ngot:\n%s", out)
	}
}

// TestImportBrewfileImportAndWrite asserts the write path reports the count and
// actually persists packages into the config file.
func TestImportBrewfileImportAndWrite(t *testing.T) {
	dir, cfgPath := setupImportTest(t)
	brewPath := writeImportBrewfile(t, dir)

	var buf bytes.Buffer
	importDryRun = false

	if err := runImport("brewfile", brewPath, &buf); err != nil {
		t.Fatalf("runImport: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "imported 3 new items") {
		t.Errorf("expected 'imported 3 new items' in output\ngot:\n%s", out)
	}
	if !strings.Contains(out, cfgPath) {
		t.Errorf("expected output to mention config path %s\ngot:\n%s", cfgPath, out)
	}

	// Config file should now contain the imported packages
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"brew: ripgrep", "brew: fd", "brew/cask: visual-studio-code"} {
		if !strings.Contains(s, want) {
			t.Errorf("config file missing %q after import", want)
		}
	}
}

// TestImportMissingConfigReturnsError asserts a config load failure surfaces
// as an error from runImport (not a panic or silent success).
func TestImportMissingConfigReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "does-not-exist.yml")
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = "" })

	var buf bytes.Buffer
	err := runImport("brewfile", "", &buf)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
	if !strings.Contains(err.Error(), "import:") {
		t.Errorf("error should be wrapped as 'import:', got: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("no output expected on config load failure, got: %q", buf.String())
	}
}

// TestImportRefusesSelfConfigTemplate (session #129): a chezmoi source dir
// managing nestor's own config imports a template whose dest is
// ~/.config/nestor/nestor.yml. The old code merged and wrote it unvalidated —
// probed: runImport returned nil and the dest landed in nestor.yml, and the
// next config load refused the file, bricking every nestor command. The merge
// is now validated through cfg.Validate() before writeConfig; on refusal the
// config stays byte-for-byte as it was.
func TestImportRefusesSelfConfigTemplate(t *testing.T) {
	dir, cfgPath := setupImportTest(t)
	src := filepath.Join(dir, "chezmoi")
	if err := os.MkdirAll(filepath.Join(src, "dot_config", "nestor"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "dot_config", "nestor", "nestor.yml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err = runImport("chezmoi", src, &buf)
	if err == nil {
		t.Fatalf("expected import of nestor's own config to be refused, got nil; output:\n%s", buf.String())
	}
	if !strings.Contains(err.Error(), "invalid config") {
		t.Errorf("error should name the invalid merged config, got: %v", err)
	}

	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("refused import must leave the config byte-for-byte unchanged")
	}
	if strings.Contains(string(after), ".config/nestor/nestor.yml") {
		t.Error("the colliding dest must not be written into the config")
	}
}

// TestImportValidChezmoiStillWrites pins the untouched path: a normal chezmoi
// dir with regular dotfiles still merges and writes exactly as before.
func TestImportValidChezmoiStillWrites(t *testing.T) {
	dir, cfgPath := setupImportTest(t)
	src := filepath.Join(dir, "chezmoi")
	if err := os.MkdirAll(filepath.Join(src, "dot_config", "nvim"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "dot_config", "nvim", "init.toml"), []byte("[ok]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runImport("chezmoi", src, &buf); err != nil {
		t.Fatalf("valid import should succeed, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "imported 1 new items") {
		t.Errorf("output should report the import, got:\n%s", out)
	}
	raw, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(raw), ".config/nvim/init.toml") {
		t.Error("the imported dest should be written into the config")
	}
}

// TestImportMaterializesTemplates: import used to record src entries and copy
// nothing, so every imported dotfile was src-missing from the first 'nestor
// up' on (probed with the HEAD binary: exit 0, empty source dir, 'diff'
// reporting "src missing"). Each new template must land in the source dir
// with the origin's bytes; a same-basename pair keeps distinct srcs, and an
// orphan already on disk is never adopted or overwritten.
func TestImportMaterializesTemplates(t *testing.T) {
	dir, cfgPath := setupImportTest(t)
	t.Setenv("HOME", dir)
	src := filepath.Join(dir, "chezmoi")
	files := map[string]string{
		"dot_bashrc":                "export FROM_CHEZMOI=1\n",
		"dot_config/git/config":     "[user]\n\tname = git\n",
		"private_dot_ssh/config":    "Host *\n",
		"dot_config/nvim/init.toml": "[ok]\n",
	}
	for rel, body := range files {
		p := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// dot_bashrc is plain, so its src is .bashrc (2026-11-22): the orphan
	// sits on that name to keep exercising the collision path.
	orphan := filepath.Join(dir, "dotfiles", ".bashrc")
	if err := os.WriteFile(orphan, []byte("ORPHAN\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runImport("chezmoi", src, &buf); err != nil {
		t.Fatalf("import should succeed, got: %v\n%s", err, buf.String())
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("imported config must load: %v", err)
	}
	want := map[string]string{
		"~/.bashrc":                files["dot_bashrc"],
		"~/.config/git/config":     files["dot_config/git/config"],
		"~/.ssh/config":            files["private_dot_ssh/config"],
		"~/.config/nvim/init.toml": files["dot_config/nvim/init.toml"],
	}
	if len(cfg.Dotfiles.Templates) != len(want) {
		t.Fatalf("want %d templates, got %+v", len(want), cfg.Dotfiles.Templates)
	}
	srcs := map[string]bool{}
	for _, tm := range cfg.Dotfiles.Templates {
		body, ok := want[tm.Dest]
		if !ok {
			t.Errorf("unexpected dest %s", tm.Dest)
			continue
		}
		if srcs[tm.Src] {
			t.Errorf("src %s recorded for two dests", tm.Src)
		}
		srcs[tm.Src] = true
		got, err := os.ReadFile(filepath.Join(cfg.Dotfiles.Source, tm.Src))
		if err != nil {
			t.Errorf("template for %s not materialized: %v", tm.Dest, err)
			continue
		}
		if string(got) != body {
			t.Errorf("template for %s = %q, want origin bytes %q", tm.Dest, got, body)
		}
		if tm.Dest == "~/.bashrc" && tm.Src == ".bashrc" {
			t.Error("the orphan .bashrc on disk must not be adopted")
		}
	}
	if got, _ := os.ReadFile(orphan); string(got) != "ORPHAN\n" {
		t.Errorf("orphan template must be left untouched, got %q", got)
	}
}

// TestImportRefusesUnreadableOrigin: an origin that cannot be read (here a
// dangling symlink in the chezmoi dir) refuses the whole import before any
// copy, so neither the config nor the source dir changes.
func TestImportRefusesUnreadableOrigin(t *testing.T) {
	dir, cfgPath := setupImportTest(t)
	t.Setenv("HOME", dir)
	src := filepath.Join(dir, "chezmoi")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "dot_zshrc"), []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(src, "dot_bashrc")); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(cfgPath)

	var buf bytes.Buffer
	err := runImport("chezmoi", src, &buf)
	if err == nil || !strings.Contains(err.Error(), "nothing written") {
		t.Fatalf("expected a loud refusal, got: %v", err)
	}
	if after, _ := os.ReadFile(cfgPath); string(after) != string(before) {
		t.Error("refused import must leave the config byte-for-byte unchanged")
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "dotfiles"))
	if len(entries) != 0 {
		t.Errorf("refused import must copy nothing, source dir has %d entries", len(entries))
	}
}

// TestImportPlainDotfileDeploysVerbatim pins the 2026-11-22 fix: import named
// every dotfile <name>.tmpl, so deploy ran plain files through text/template.
// A chezmoi .vimrc with "{{{" fold markers failed with `unexpected "{"` and
// never reached HOME (probed with the HEAD binary). Plain files now import
// under their own name and deploy byte for byte; a file chezmoi marks as a
// template (dot_x.tmpl) stays a nestor template and is still rendered.
func TestImportPlainDotfileDeploysVerbatim(t *testing.T) {
	dir, cfgPath := setupImportTest(t)
	t.Setenv("HOME", dir)
	t.Setenv("NESTOR_IMPORT_PROBE", "rendered")
	src := filepath.Join(dir, "chezmoi")
	vimrc := "\" General {{{\nset number\n\" }}}\n"
	files := map[string]string{
		"dot_vimrc":            vimrc,
		"dot_probe.tmpl":       "v={{ env \"NESTOR_IMPORT_PROBE\" }}\n",
		"dot_raw.tmpl.literal": "{{ not a template }}\n",
	}
	for rel, body := range files {
		p := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	if err := runImport("chezmoi", src, &buf); err != nil {
		t.Fatalf("import: %v\n%s", err, buf.String())
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("imported config must load: %v", err)
	}
	wantSrc := map[string]string{
		"~/.vimrc":    ".vimrc",
		"~/.probe":    ".probe.tmpl",
		"~/.raw.tmpl": ".raw.tmpl.literal",
	}
	var temps []dotfiles.Template
	for _, tm := range cfg.Dotfiles.Templates {
		if want, ok := wantSrc[tm.Dest]; !ok || tm.Src != want {
			t.Errorf("dest %s: src %q, want %q", tm.Dest, tm.Src, want)
		}
		temps = append(temps, dotfiles.Template{Src: tm.Src, Dest: tm.Dest})
	}
	if len(temps) != len(wantSrc) {
		t.Fatalf("want %d templates, got %+v", len(wantSrc), cfg.Dotfiles.Templates)
	}

	d := dotfiles.Deployer{Strategy: dotfiles.StrategyCopy, Source: cfg.Dotfiles.Source}
	for _, r := range d.DeployAll(temps) {
		if r.Err != nil {
			t.Errorf("deploy %s: %v", r.Template.Dest, r.Err)
		}
	}
	wantBody := map[string]string{
		".vimrc":    vimrc,
		".probe":    "v=rendered\n",
		".raw.tmpl": "{{ not a template }}\n",
	}
	for name, want := range wantBody {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Errorf("~/%s: got %q (err %v), want %q", name, got, err, want)
		}
	}
}
