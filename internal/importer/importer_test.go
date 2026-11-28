package importer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

func TestBrewfileImport(t *testing.T) {
	dir := t.TempDir()
	brewfile := `# my brewfile
brew "git"
brew "ripgrep"
cask "visual-studio-code"
tap "hashicorp/tap"
mas "Xcode", id: 497799835
brew "neovim"
`
	path := filepath.Join(dir, "Brewfile")
	if err := os.WriteFile(path, []byte(brewfile), 0o644); err != nil {
		t.Fatal(err)
	}

	bf, err := NewBrewfile(path)
	if err != nil {
		t.Fatalf("NewBrewfile: %v", err)
	}

	res, err := bf.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	// git, ripgrep, neovim (brew) + visual-studio-code (cask) = 4; the tap
	// and the mas entry are skipped — a tap is a formula repository, not an
	// installable package, and no manager dispatches "homebrew/tap".
	wantPkgs := 4
	if got := len(res.Packages); got != wantPkgs {
		t.Errorf("packages: got %d (%v), want %d", got, res.Packages, wantPkgs)
	}

	// mas entry + tap entry should be skipped
	if res.Skipped != 2 {
		t.Errorf("skipped: got %d, want 2 (mas + tap entries)", res.Skipped)
	}

	// verify canonical brew: prefix on brew entries (the dialect the
	// packages dispatcher actually speaks)
	for _, p := range res.Packages {
		switch {
		case strings.Contains(p, "git"):
			if p != "brew: git" {
				t.Errorf("git package: got %q, want %q", p, "brew: git")
			}
		case strings.Contains(p, "visual-studio-code"):
			if p != "brew/cask: visual-studio-code" {
				t.Errorf("cask: got %q", p)
			}
		}
	}
}

func TestBrewfileTapIsNotAPackage(t *testing.T) {
	// Regression: taps used to be emitted as "homebrew/tap: <name>"
	// package specs. No backend dispatches that manager, so every
	// imported config failed 'nestor up' with "unsupported package
	// manager: homebrew/tap". Taps are formula repositories brew pulls
	// from on demand — they must not enter the package list.
	dir := t.TempDir()
	brewfile := "tap \"hashicorp/tap\"\ntap \"homebrew/bundle\"\n"
	path := filepath.Join(dir, "Brewfile")
	if err := os.WriteFile(path, []byte(brewfile), 0o644); err != nil {
		t.Fatal(err)
	}
	bf, err := NewBrewfile(path)
	if err != nil {
		t.Fatalf("NewBrewfile: %v", err)
	}
	res, err := bf.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Packages) != 0 {
		t.Errorf("packages: got %v, want none (taps are not packages)", res.Packages)
	}
	if res.Skipped != 2 {
		t.Errorf("skipped: got %d, want 2 (both taps)", res.Skipped)
	}
}

func TestBrewfileMerge(t *testing.T) {
	dir := t.TempDir()
	brewfile := `brew "git"
brew "neovim"
`
	path := filepath.Join(dir, "Brewfile")
	os.WriteFile(path, []byte(brewfile), 0o644)

	bf, _ := NewBrewfile(path)
	res, _ := bf.Import()

	// config already has the LEGACY "homebrew: git" spelling (what the
	// brewfile importer wrote before import and install spoke the same
	// dialect). Merge dedups by parsed spec, so the canonical "brew: git"
	// the importer now produces must NOT duplicate beside it.
	cfg := &config.Config{
		Packages: config.Packages{
			Common: []string{"homebrew: git", "homebrew: curl"},
		},
	}

	added, mergeErr := MergeResult(cfg, res)
	if mergeErr != nil {
		t.Fatalf("MergeResult: %v", mergeErr)
	}
	if added != 1 {
		t.Errorf("added: got %d, want 1 (neovim is new; legacy homebrew: git dedups against brew: git)", added)
	}
	for _, p := range cfg.Packages.Common {
		if p == "brew: git" {
			t.Errorf("canonical git entry duplicated beside its legacy spelling: %v", cfg.Packages.Common)
		}
	}

	// verify neovim was added
	found := false
	for _, p := range cfg.Packages.Common {
		if p == "brew: neovim" {
			found = true
		}
	}
	if !found {
		t.Errorf("neovim not found in config after merge, got: %v", cfg.Packages.Common)
	}
}

func TestChezmoiImport(t *testing.T) {
	dir := t.TempDir()
	chezmoiDir := filepath.Join(dir, "chezmoi")
	files := map[string]string{
		"dot_bashrc":               "# bash config\n",
		"dot_gitconfig":            "[user]\n",
		"dot_vimrc":                "\" vim\n",
		"dot_config/nvim/init.vim": "\" nvim\n",
		"private_dot_ssh/config":   "Host github\n",
		".git/info/exclude":        "",
		"README.md":                "# readme\n",
	}
	for relPath, content := range files {
		full := filepath.Join(chezmoiDir, relPath)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}

	cz, err := NewChezmoi(chezmoiDir)
	if err != nil {
		t.Fatalf("NewChezmoi: %v", err)
	}

	res, err := cz.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(res.Dotfiles) != 5 {
		t.Errorf("dotfiles: got %d, want 5 (bashrc, gitconfig, vimrc, nvim/init.vim, ssh/config). got: %v",
			len(res.Dotfiles), res.Dotfiles)
	}

	// verify a decode
	foundBashrc := false
	for _, d := range res.Dotfiles {
		if d.Dest == "~/.bashrc" {
			foundBashrc = true
			// plain chezmoi file: copied, never rendered (2026-11-22)
			if d.Src != ".bashrc" {
				t.Errorf("bashrc src: got %q, want %q", d.Src, ".bashrc")
			}
		}
	}
	if !foundBashrc {
		t.Errorf("~/.bashrc not found in imports: %v", res.Dotfiles)
	}
}

func TestChezmoiPathDecode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"dot_bashrc", ".bashrc"},
		{"dot_config/nvim/init.vim", ".config/nvim/init.vim"},
		{"private_dot_ssh/config", ".ssh/config"},
		{"exact_dot_config/fish/config.fish", ".config/fish/config.fish"},
	}
	for _, tt := range tests {
		got := decodeChezmoiPath(tt.input)
		if got != tt.want {
			t.Errorf("decodeChezmoiPath(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMergeResultDedup(t *testing.T) {
	res := &Result{
		Packages: []string{"git", "neovim"},
		Dotfiles: []config.Template{
			{Src: ".bashrc.tmpl", Dest: "~/.bashrc"},
			{Src: ".zshrc.tmpl", Dest: "~/.zshrc"},
		},
	}

	cfg := &config.Config{
		Packages: config.Packages{
			Common: []string{"git"},
		},
		Dotfiles: config.Dotfiles{
			Templates: []config.Template{
				{Src: ".bashrc.tmpl", Dest: "~/.bashrc"},
			},
		},
	}

	added, mergeErr := MergeResult(cfg, res)
	if mergeErr != nil {
		t.Fatalf("MergeResult: %v", mergeErr)
	}
	if added != 2 { // neovim (pkg) + zshrc (dotfile)
		t.Errorf("added: got %d, want 2", added)
	}

	if len(cfg.Packages.Common) != 2 {
		t.Errorf("packages after merge: got %d, want 2", len(cfg.Packages.Common))
	}
	if len(cfg.Dotfiles.Templates) != 2 {
		t.Errorf("dotfiles after merge: got %d, want 2", len(cfg.Dotfiles.Templates))
	}
}

func TestBrewfileNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := NewBrewfile(filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Error("expected error for nonexistent Brewfile, got nil")
	}
}

func TestChezmoiNotFound(t *testing.T) {
	_, err := NewChezmoi("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent chezmoi dir, got nil")
	}
}

// Two imported files sharing a basename (and an import over a name the
// config already uses for another file) must not share a template src:
// both dests would deploy the same bytes on 'nestor up'.
func TestMergeResultSameBasenameGetsOwnSrc(t *testing.T) {
	cfg := &config.Config{
		Dotfiles: config.Dotfiles{
			Templates: []config.Template{
				{Src: "config.tmpl", Dest: "~/.config/fish/config"},
			},
		},
		Profiles: map[string]config.Profile{
			"work": {Dotfiles: []config.Template{{Src: "config-2.tmpl", Dest: "~/.work/config"}}},
		},
	}
	res := &Result{Dotfiles: []config.Template{
		{Src: "config.tmpl", Dest: "~/.config/git/config"},
		{Src: "config.tmpl", Dest: "~/.ssh/config"},
		{Src: ".bashrc.tmpl", Dest: "~/.bashrc"},
	}}
	added, err := MergeResult(cfg, res)
	if err != nil {
		t.Fatalf("MergeResult: %v", err)
	}
	if added != 3 {
		t.Fatalf("added: got %d, want 3", added)
	}
	want := map[string]string{
		"~/.config/fish/config": "config.tmpl",
		"~/.config/git/config":  "config-3.tmpl",
		"~/.ssh/config":         "config-4.tmpl",
		"~/.bashrc":             ".bashrc.tmpl",
	}
	seen := map[string]string{}
	for _, d := range cfg.Dotfiles.Templates {
		if w, ok := want[d.Dest]; !ok || d.Src != w {
			t.Errorf("dest %s: src %q, want %q", d.Dest, d.Src, want[d.Dest])
		}
		if prev, dup := seen[d.Src]; dup {
			t.Errorf("src %s shared by %s and %s", d.Src, prev, d.Dest)
		}
		seen[d.Src] = d.Dest
	}
}

// A dest the config already declares is still deduped, keeping its src.
func TestMergeResultExistingDestKeepsSrc(t *testing.T) {
	cfg := &config.Config{Dotfiles: config.Dotfiles{Templates: []config.Template{
		{Src: "config.tmpl", Dest: "~/.ssh/config"},
	}}}
	res := &Result{Dotfiles: []config.Template{{Src: "config.tmpl", Dest: "~/.ssh/config"}}}
	added, err := MergeResult(cfg, res)
	if err != nil {
		t.Fatalf("MergeResult: %v", err)
	}
	if added != 0 || len(cfg.Dotfiles.Templates) != 1 || cfg.Dotfiles.Templates[0].Src != "config.tmpl" {
		t.Fatalf("re-import changed config: added=%d templates=%+v", added, cfg.Dotfiles.Templates)
	}
}

func TestChezmoiPathDecodeStripsTemplateSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"dot_gitconfig.tmpl", ".gitconfig"},
		{"dot_config/git/private_ignore.tmpl", ".config/git/ignore"},
		{"private_executable_dot_local/bin/tool.tmpl", ".local/bin/tool"},
		{"dot_vimrc.tmpl.literal", ".vimrc.tmpl"},
		// only the target file's own suffix is an attribute
		{"dot_config/app.tmpl/settings", ".config/app.tmpl/settings"},
		{"dot_bashrc", ".bashrc"},
	}
	for _, tt := range tests {
		if got := decodeChezmoiPath(tt.input); got != tt.want {
			t.Errorf("decodeChezmoiPath(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestChezmoiImportTemplateDest(t *testing.T) {
	dir := t.TempDir()
	origin := filepath.Join(dir, "dot_gitconfig.tmpl")
	if err := os.WriteFile(origin, []byte("[user]\n  name = {{ env \"USER\" }}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cz, err := NewChezmoi(dir)
	if err != nil {
		t.Fatalf("NewChezmoi: %v", err)
	}
	res, err := cz.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Dotfiles) != 1 {
		t.Fatalf("dotfiles: got %v, want one", res.Dotfiles)
	}
	d := res.Dotfiles[0]
	if d.Dest != "~/.gitconfig" || d.Src != ".gitconfig.tmpl" {
		t.Errorf("got dest %q src %q, want ~/.gitconfig on .gitconfig.tmpl", d.Dest, d.Src)
	}
	if res.Origins["~/.gitconfig"] != origin {
		t.Errorf("origin for ~/.gitconfig: got %q, want %q (origins %v)", res.Origins["~/.gitconfig"], origin, res.Origins)
	}
}

// TestImportSrcNaming pins how imported srcs keep their kind (2026-11-22):
// only a chezmoi template becomes <name>.tmpl, a plain name that happens to
// end in .tmpl gets .literal so deploy copies it, and collision numbering
// keeps the template-or-plain suffix.
func TestImportSrcNaming(t *testing.T) {
	isTmpl := map[string]bool{
		"dot_gitconfig.tmpl":     true,
		"dot_vimrc":              false,
		"dot_vimrc.tmpl.literal": false,
		".tmpl":                  false,
	}
	for base, want := range isTmpl {
		if got := chezmoiIsTemplate(base); got != want {
			t.Errorf("chezmoiIsTemplate(%q) = %v, want %v", base, got, want)
		}
	}
	srcs := []struct {
		name string
		tmpl bool
		want string
	}{
		{".gitconfig", true, ".gitconfig.tmpl"},
		{".vimrc", false, ".vimrc"},
		{".vimrc.tmpl", false, ".vimrc.tmpl.literal"},
	}
	for _, c := range srcs {
		if got := importSrc(c.name, c.tmpl); got != c.want {
			t.Errorf("importSrc(%q, %v) = %q, want %q", c.name, c.tmpl, got, c.want)
		}
	}
	numbered := map[string]string{
		"config.tmpl":         "config-2.tmpl",
		"sub/config.tmpl":     "sub/config-2.tmpl",
		".vimrc":              ".vimrc-2",
		".vimrc.tmpl.literal": ".vimrc.tmpl.literal-2",
	}
	for in, want := range numbered {
		if got := NumberedSrc(in, 2); got != want {
			t.Errorf("NumberedSrc(%q, 2) = %q, want %q", in, got, want)
		}
	}
}

// chezmoi ignores every source name starting with a literal "." at any
// depth. Only ".git" was skipped, so chezmoi's own .chezmoiignore,
// .chezmoi.toml.tmpl, .chezmoiscripts/ and .chezmoitemplates/ were imported
// as dotfiles and 'up' wrote them into HOME (probed 2026-11-27; the expected
// set below is what chezmoi v2.74.2 'managed --include files' lists).
func TestChezmoiImportSkipsChezmoiOwnFiles(t *testing.T) {
	chezmoiDir := filepath.Join(t.TempDir(), "chezmoi")
	files := map[string]string{
		"dot_bashrc":                          "alias ll='ls -l'\n",
		"dot_config/app/conf":                 "k=v\n",
		".chezmoiignore":                      "README.md\n",
		".chezmoi.toml.tmpl":                  "[data]\n",
		".chezmoiversion":                     "2.0.0\n",
		".chezmoiscripts/run_once_install.sh": "#!/bin/sh\n",
		".chezmoitemplates/footer":            "shared\n",
		".editorconfig":                       "root = true\n",
		".github/workflows/ci.yml":            "on: push\n",
		"dot_config/app/.hidden":              "x\n",
	}
	for rel, content := range files {
		full := filepath.Join(chezmoiDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cz, err := NewChezmoi(chezmoiDir)
	if err != nil {
		t.Fatalf("NewChezmoi: %v", err)
	}
	res, err := cz.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	var dests []string
	for _, d := range res.Dotfiles {
		dests = append(dests, d.Dest)
		if strings.Contains(d.Dest, ".chezmoi") {
			t.Errorf("chezmoi's own file imported as a dotfile: %+v", d)
		}
	}
	want := map[string]bool{"~/.bashrc": true, "~/.config/app/conf": true}
	if len(dests) != len(want) {
		t.Fatalf("dests: got %v, want exactly ~/.bashrc and ~/.config/app/conf", dests)
	}
	for _, d := range dests {
		if !want[d] {
			t.Errorf("unexpected dest %q (chezmoi does not manage it)", d)
		}
	}
	if res.Skipped != 8 {
		t.Errorf("skipped: got %d, want 8 (every dot-named source)", res.Skipped)
	}
	if len(res.Origins) != 2 {
		t.Errorf("origins: got %v, want only the 2 managed files", res.Origins)
	}
}

func TestChezmoiPathDecodeSkipsDotNames(t *testing.T) {
	for _, in := range []string{
		".chezmoiignore",
		".chezmoi.toml.tmpl",
		".chezmoiscripts/run_once_install.sh",
		".chezmoitemplates/footer",
		".editorconfig",
		"dot_config/app/.hidden",
	} {
		if got := decodeChezmoiPath(in); got != "__skip__" {
			t.Errorf("decodeChezmoiPath(%q): got %q, want __skip__", in, got)
		}
	}
	// dot_ is chezmoi's spelling of a leading dot and must still decode.
	if got := decodeChezmoiPath("private_dot_ssh/config"); got != ".ssh/config" {
		t.Errorf("private_dot_ssh/config: got %q, want .ssh/config", got)
	}
	// The dot test is on the raw name: chezmoi v2.74.2 manages
	// private_.secret as ~/.secret.
	if got := decodeChezmoiPath("private_.secret"); got != ".secret" {
		t.Errorf("private_.secret: got %q, want .secret", got)
	}
}

// chezmoi source names checked against 'chezmoi managed' (v2.74.2,
// 2026-11-28): exact_ dirs still hold managed files, empty_ and literal_
// are name attributes, and run_/symlink_/modify_/remove_/encrypted_ entries
// are not file contents nestor can deploy.
func TestChezmoiPathDecodeSpecialPrefixes(t *testing.T) {
	tests := []struct{ input, want string }{
		{"exact_dot_fish/config.fish", ".fish/config.fish"},
		{"dot_vim/exact_colors/x.vim", ".vim/colors/x.vim"},
		{"private_exact_dot_ssh/config", ".ssh/config"},
		{"dot_config/empty_blank", ".config/blank"},
		{"literal_dot_keep", ""},
		{"dot_config/literal_dot_keep", ".config/dot_keep"},
		{"dot_config/exact_note", ".config/exact_note"},
		{"dot_config/run_once_setup.sh", "__skip__"},
		{"dot_config/symlink_link", "__skip__"},
		{"dot_config/modify_mod", "__skip__"},
		{"dot_config/remove_gone", "__skip__"},
		{"remove_dot_old/file", "__skip__"},
		{"encrypted_private_dot_netrc", "__skip__"},
		{"create_encrypted_dot_token", "__skip__"},
		{"create_dot_hushlogin", ".hushlogin"},
	}
	for _, tt := range tests {
		if got := decodeChezmoiPath(tt.input); got != tt.want {
			t.Errorf("decodeChezmoiPath(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

// The import as a whole: what 'chezmoi managed --include files' lists for
// this tree is exactly what nestor imports, and nothing else.
func TestChezmoiImportMatchesManagedFiles(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"dot_bashrc":                   "export A=1\n",
		"exact_dot_fish/config.fish":   "set -x A 1\n",
		"dot_vim/exact_colors/x.vim":   "hi\n",
		"dot_config/empty_blank":       "x\n",
		"dot_config/run_once_setup.sh": "#!/bin/sh\n",
		"dot_config/symlink_link":      "target\n",
		"dot_config/modify_mod":        "#!/bin/sh\ncat\n",
		"dot_config/remove_gone":       "y\n",
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

	res, err := (&Chezmoi{SourceDir: src}).Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	var got []string
	for _, d := range res.Dotfiles {
		got = append(got, d.Dest)
	}
	sort.Strings(got)
	want := []string{"~/.bashrc", "~/.config/blank", "~/.fish/config.fish", "~/.vim/colors/x.vim"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("imported dests:\n got %v\nwant %v", got, want)
	}
	if res.Skipped != 4 {
		t.Errorf("skipped: got %d, want 4 (run_, symlink_, modify_, remove_)", res.Skipped)
	}
	if o := res.Origins["~/.fish/config.fish"]; o != filepath.Join(src, "exact_dot_fish", "config.fish") {
		t.Errorf("origin for exact_ file: got %q", o)
	}
}
