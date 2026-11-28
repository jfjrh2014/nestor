// Package importer converts external tool configs into nestor config entries.
package importer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/packages"
	"github.com/jfjrh2014/nestor/internal/pathutil"
)

// Result holds what was imported.
type Result struct {
	Packages []string
	Dotfiles []config.Template
	Source   string
	Skipped  int
	// Origins maps each imported dotfile's Dest to the absolute file its
	// template content comes from (the chezmoi source file, or the live file
	// yadm tracks). Callers materialize templates from it: without a copy the
	// recorded src never exists and every 'nestor up' reports src-missing.
	Origins map[string]string
}

// Importer is the common interface for all import sources.
type Importer interface {
	Import() (*Result, error)
	Name() string
}

// Auto tries to detect which tool is in use and returns the right importer.
// Checks chezmoi first, then yadm, then Brewfile.
func Auto() (Importer, error) {
	if c, err := NewChezmoi(""); err == nil {
		return c, nil
	}
	if y, err := NewYadm(); err == nil {
		return y, nil
	}
	if b, err := NewBrewfile(""); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("no importable tool found (looked for chezmoi, yadm, Brewfile)")
}

// --- Brewfile ---

// Brewfile parses a Brewfile and extracts tap, brew, and cask entries.
type Brewfile struct {
	Path string
}

func NewBrewfile(path string) (*Brewfile, error) {
	if path == "" {
		// search CWD and ~
		for _, candidate := range []string{"Brewfile", "~/Brewfile"} {
			expanded := expandHome(candidate)
			if _, err := os.Stat(expanded); err == nil {
				path = expanded
				break
			}
		}
	}
	if path == "" {
		// try $HOME/.Brewfile as last resort
		if home, err := osUserHomeDir(); err == nil {
			candidate := filepath.Join(home, ".Brewfile")
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
	}
	if path == "" {
		return nil, fmt.Errorf("no Brewfile found")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("Brewfile not found: %w", err)
	}
	return &Brewfile{Path: path}, nil
}

func (b *Brewfile) Name() string { return "brewfile" }

func (b *Brewfile) Import() (*Result, error) {
	f, err := os.Open(b.Path)
	if err != nil {
		return nil, fmt.Errorf("opening Brewfile: %w", err)
	}
	defer f.Close()

	res := &Result{Source: b.Path}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		// parse "brew 'name'", "tap 'user/repo'", "cask 'name'"
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		kind := parts[0]
		name := strings.Trim(parts[1], `"'`)

		switch kind {
		case "tap":
			// A tap is a formula repository, not an installable package —
			// brew pulls from it on demand when a formula or cask needs
			// it. Listing it as a package made every import unrunnable:
			// no manager dispatches "homebrew/tap".
			res.Skipped++
		case "brew":
			// Canonical spec ("brew", not "homebrew") — packages.ParseSpec
			// maps the legacy alias for configs written by older imports.
			res.Packages = append(res.Packages, "brew: "+name)
		case "cask":
			res.Packages = append(res.Packages, "brew/cask: "+name)
		case "mas":
			// Mac App Store - skip
			res.Skipped++
		}
	}
	return res, scanner.Err()
}

// --- chezmoi ---

// Chezmoi scans a chezmoi source directory for managed dotfiles.
type Chezmoi struct {
	SourceDir string
}

func NewChezmoi(sourceDir string) (*Chezmoi, error) {
	if sourceDir == "" {
		home, err := osUserHomeDir()
		if err != nil {
			return nil, err
		}
		sourceDir = filepath.Join(home, ".local", "share", "chezmoi")
	}
	if _, err := os.Stat(sourceDir); err != nil {
		return nil, fmt.Errorf("chezmoi source dir not found: %w", err)
	}
	return &Chezmoi{SourceDir: sourceDir}, nil
}

func (c *Chezmoi) Name() string { return "chezmoi" }

func (c *Chezmoi) Import() (*Result, error) {
	res := &Result{Source: c.SourceDir, Origins: map[string]string{}}

	err := filepath.Walk(c.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(c.SourceDir, path)
		// chezmoi encodes dotfiles with a leading "dot_" prefix
		// e.g. dot_bashrc -> .bashrc, dot_config/nvim/init.toml -> .config/nvim/init.toml
		dest := decodeChezmoiPath(rel)
		if dest == "" || dest == "__skip__" {
			res.Skipped++
			return nil
		}

		// Only files chezmoi itself renders (a ".tmpl" attribute not ended by
		// ".literal") become nestor templates. Naming every import <name>.tmpl
		// made 'up' run plain files through text/template: a .vimrc with
		// "{{{" fold markers failed with `unexpected "{"` and never deployed
		// (probed 2026-11-22), and a file whose braces did parse was rewritten.
		res.Dotfiles = append(res.Dotfiles, config.Template{
			Src:  importSrc(filepath.Base(dest), chezmoiIsTemplate(filepath.Base(rel))),
			Dest: "~/" + dest,
		})
		res.Origins["~/"+dest] = path
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning chezmoi dir: %w", err)
	}
	return res, nil
}

// chezmoi attribute prefixes that stack on top of dot_/exact_.
// Order matters: strip these first, then decode the core prefix.
var chezmoiAttrPrefixes = []string{"private_", "executable_", "readonly_", "create_", "empty_"}

// skipChezmoiEntry reports whether a raw source name is a chezmoi entry
// whose contents are not the target file's contents. The type prefixes lead
// the name (before encrypted_/private_/...), so they are checked on the raw
// part; encrypted_ may follow create_.
func skipChezmoiEntry(part string, last bool) bool {
	if strings.HasPrefix(part, "remove_") {
		return true
	}
	if !last {
		return false
	}
	for _, prefix := range []string{"run_", "symlink_", "modify_", "encrypted_", "create_encrypted_"} {
		if strings.HasPrefix(part, prefix) {
			return true
		}
	}
	return false
}

// decodeChezmoiPath converts a chezmoi source path to its target home-dir path.
// Returns "" if not importable, "__skip__" for intentionally skipped files.
func decodeChezmoiPath(rel string) string {
	parts := strings.Split(rel, string(filepath.Separator))
	var decoded []string
	for i, part := range parts {
		last := i == len(parts)-1
		// chezmoi ignores every source name that starts with a literal "."
		// at any depth: its own .chezmoiignore, .chezmoi.toml.tmpl,
		// .chezmoiscripts/, .chezmoitemplates/, .chezmoiversion, plus .git,
		// .github and the like. Only ".git" used to be skipped, so the rest
		// were imported and 'up' wrote ~/.chezmoiignore etc. into home
		// (probed 2026-11-27 against chezmoi v2.74.2 'managed'). The test is
		// on the raw name: private_.secret is managed, as ~/.secret.
		if strings.HasPrefix(part, ".") {
			return "__skip__"
		}

		// Entries that are not plain file contents can't become a nestor
		// template: run_ scripts, symlink_ (contents are the link target),
		// modify_ (a script that edits the target), remove_ (a deletion
		// order) and encrypted_ (ciphertext). They used to import under
		// their raw names, e.g. ~/.config/run_once_setup.sh, and 'up' would
		// deploy them as files (probed 2026-11-28 against chezmoi v2.74.2).
		if skipChezmoiEntry(part, last) {
			return "__skip__"
		}

		// chezmoi attribute prefixes stack: private_executable_dot_foo -> .foo.
		// exact_ is a directory attribute (prune unmanaged files); the files
		// inside are still managed, so it is stripped, not skipped. Skipping
		// it dropped every file under exact_dot_config/ from the import.
		// literal_ ends prefix parsing.
		p := part
		literal := false
		for !literal {
			stripped := false
			if strings.HasPrefix(p, "literal_") {
				p = strings.TrimPrefix(p, "literal_")
				literal = true
				break
			}
			if !last && strings.HasPrefix(p, "exact_") {
				p = strings.TrimPrefix(p, "exact_")
				stripped = true
			}
			for _, prefix := range chezmoiAttrPrefixes {
				if strings.HasPrefix(p, prefix) {
					p = strings.TrimPrefix(p, prefix)
					stripped = true
					break
				}
			}
			if !stripped {
				break
			}
		}

		if !literal && strings.HasPrefix(p, "dot_") {
			p = "." + strings.TrimPrefix(p, "dot_")
		}
		decoded = append(decoded, p)
	}

	// Only import dotfiles (things starting with .) - skip arbitrary files
	if len(decoded) == 0 {
		return ""
	}
	decoded[len(decoded)-1] = stripChezmoiSuffix(decoded[len(decoded)-1])
	joined := filepath.Join(decoded...)
	if !strings.HasPrefix(decoded[0], ".") {
		return ""
	}
	return joined
}

// stripChezmoiSuffix drops chezmoi's target-name suffix attributes from the
// final path component: dot_gitconfig.tmpl manages ~/.gitconfig, not
// ~/.gitconfig.tmpl. ".literal" ends suffix parsing, so foo.tmpl.literal
// manages foo.tmpl. A bare ".tmpl"/".literal" name has no target to keep.
func stripChezmoiSuffix(name string) string {
	for _, suf := range []string{".literal", ".tmpl"} {
		if strings.HasSuffix(name, suf) && len(name) > len(suf) {
			return strings.TrimSuffix(name, suf)
		}
	}
	return name
}

// chezmoiIsTemplate reports whether chezmoi renders the source file: its
// final component carries the ".tmpl" suffix attribute and ".literal" did not
// end suffix parsing first (foo.tmpl.literal is the plain file foo.tmpl).
func chezmoiIsTemplate(base string) bool {
	if strings.HasSuffix(base, ".literal") {
		return false
	}
	return strings.HasSuffix(base, ".tmpl") && len(base) > len(".tmpl")
}

// importSrc names an imported dotfile's source: <name>.tmpl for a file the
// foreign tool treats as a template, the bare name for a plain file. Deploy
// renders only .tmpl srcs, so a plain file is copied byte for byte. A plain
// file whose own name ends in ".tmpl" (chezmoi's foo.tmpl.literal, or a yadm
// file named so) gets ".literal" appended, or deploy would render it anyway.
func importSrc(name string, template bool) string {
	if template {
		return name + ".tmpl"
	}
	if strings.HasSuffix(name, ".tmpl") {
		return name + ".literal"
	}
	return name
}

// PlainSrc names a source for a file that must be copied, not rendered: the
// name itself, or name+".literal" when it already ends in ".tmpl".
func PlainSrc(name string) string {
	return importSrc(name, false)
}

// NumberedSrc returns src with "-n" inserted before a ".tmpl" suffix
// (config.tmpl -> config-2.tmpl) or appended to a plain name (.vimrc ->
// .vimrc-2), so a renamed src keeps its template-or-plain kind.
func NumberedSrc(src string, n int) string {
	dir, file := filepath.Split(src)
	if strings.HasSuffix(file, ".tmpl") && len(file) > len(".tmpl") {
		return dir + fmt.Sprintf("%s-%d.tmpl", strings.TrimSuffix(file, ".tmpl"), n)
	}
	return dir + fmt.Sprintf("%s-%d", file, n)
}

// --- yadm ---

// Yadm uses `yadm list -a` to find managed files.
type Yadm struct{}

func NewYadm() (*Yadm, error) {
	// check if yadm is installed
	if _, err := execLookPath("yadm"); err != nil {
		return nil, fmt.Errorf("yadm not found in PATH")
	}
	return &Yadm{}, nil
}

func (y *Yadm) Name() string { return "yadm" }

func (y *Yadm) Import() (*Result, error) {
	// yadm list -a prints every managed file relative to the work tree
	out, err := runCommand("yadm", "list", "-a")
	if err != nil {
		return nil, fmt.Errorf("running yadm list: %w", err)
	}

	res := &Result{Source: "yadm", Origins: map[string]string{}}
	// The "~/"-relative dests below only mean anything against the real
	// home: a failed lookup would TrimPrefix("")+"/" off every absolute
	// path, turning managed files into "~/home/u/..." and paths outside
	// home (e.g. /tmp/...) into in-home dests "~/tmp/..." — mappings
	// pointing at files that do not exist (probed 2026-11-05). Refuse
	// loudly, same contract as the write boundaries (2026-10-10).
	home, homeErr := osUserHomeDir()
	if homeErr != nil {
		return nil, fmt.Errorf("resolving home directory: %w", homeErr)
	}

	// `yadm list -a` is `git ls-files` run from the yadm work tree, so it
	// prints paths relative to that tree (".bashrc"), not absolute ones.
	// Treating them as absolute skipped every managed file as "outside
	// home" and the import added nothing (probed 2026-11-26 with real
	// yadm). Resolve relative lines against the work tree yadm reports.
	workTree := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		abs := line
		if !filepath.IsAbs(line) {
			if workTree == "" {
				top, err := runCommand("yadm", "rev-parse", "--show-toplevel")
				workTree = strings.TrimSpace(top)
				if err != nil || !filepath.IsAbs(workTree) {
					return nil, fmt.Errorf("resolving yadm work tree: %v", err)
				}
			}
			abs = filepath.Join(workTree, line)
		}
		abs = filepath.Clean(abs)

		rel, err := filepath.Rel(filepath.Clean(home), abs)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			// outside home dir, skip
			res.Skipped++
			continue
		}
		rel = filepath.ToSlash(rel)
		line = abs

		// yadm files are plain (yadm's own templates are ##template
		// alternates, not something nestor can render), so they copy as is.
		res.Dotfiles = append(res.Dotfiles, config.Template{
			Src:  importSrc(filepath.Base(rel), false),
			Dest: "~/" + rel,
		})
		res.Origins["~/"+rel] = line
	}
	return res, nil
}

// MergeResult merges an import result into a config.
// Deduplicates packages and dotfiles that already exist. Packages dedup by
// parsed spec (manager canonicalized, name) so a legacy "homebrew: git"
// entry from a pre-unification config doesn't duplicate beside the
// importer's canonical "brew: git". Returns the count of new items added.
// A failed home lookup returns an error instead of degrading the canonical
// dedup: with an empty home every tilde dest compares unexpanded, so a
// "~/.bashrc" import beside the absolute-spelled entry in the config
// merges as two files and two templates deploy to one dest last-wins.
func MergeResult(cfg *config.Config, res *Result) (int, error) {
	added := 0

	// packages
	pkgKey := func(raw string) string {
		s := packages.ParseSpec(raw, "")
		return s.Manager + "/" + s.Name
	}
	existingPkgs := map[string]bool{}
	for _, p := range cfg.Packages.Common {
		existingPkgs[pkgKey(p)] = true
	}
	for _, p := range res.Packages {
		k := pkgKey(p)
		if !existingPkgs[k] {
			cfg.Packages.Common = append(cfg.Packages.Common, p)
			existingPkgs[k] = true
			added++
		}
	}

	// dotfiles, deduped by canonical dest (tilde/absolute/./ spellings of
	// one file collide — raw-string keys let "./"-spelled dupes through, the
	// same gap validate()'s duplicate check was closed for 2026-11-01). Both
	// sides must resolve against the same home: with "" every tilde dest
	// compares unexpanded and a "~/.bashrc" import beside the absolute-
	// spelled entry merges as two files (the last-wins deploy corruption
	// CanonicalDest exists to prevent). Refuse loudly rather than merge
	// degraded — same contract as Yadm.Import above.
	home, homeErr := osUserHomeDir()
	if homeErr != nil {
		return 0, fmt.Errorf("merging import: resolving home directory: %w", homeErr)
	}
	existingDots := map[string]bool{}
	for _, d := range cfg.Dotfiles.Templates {
		existingDots[pathutil.CanonicalDest(d.Dest, home)] = true
	}
	// Importers name srcs <base>.tmpl, which is only unique per basename:
	// ~/.config/git/config and ~/.ssh/config both want config.tmpl, and an
	// import beside an existing entry can hit a name the config already
	// uses for another file. Two dests on one src deploy the same bytes to
	// both — the next 'nestor up' writes git's config over ~/.ssh/config
	// (probed 2026-11-13). Same rule 'add dotfile' adopted 2026-11-12: a
	// taken name falls through to the first free <base>-N.tmpl.
	usedSrcs := map[string]bool{}
	for _, d := range cfg.Dotfiles.Templates {
		usedSrcs[filepath.Clean(d.Src)] = true
	}
	for _, p := range cfg.Profiles {
		for _, d := range p.Dotfiles {
			usedSrcs[filepath.Clean(d.Src)] = true
		}
	}
	for _, d := range res.Dotfiles {
		key := pathutil.CanonicalDest(d.Dest, home)
		if existingDots[key] {
			continue
		}
		d.Src = uniqueSrc(d.Src, usedSrcs)
		usedSrcs[filepath.Clean(d.Src)] = true
		cfg.Dotfiles.Templates = append(cfg.Dotfiles.Templates, d)
		existingDots[key] = true
		added++
	}

	return added, nil
}

// uniqueSrc returns src unchanged when no other template uses it, else the
// first NumberedSrc(src, N) (N from 2) not in used.
func uniqueSrc(src string, used map[string]bool) string {
	if !used[filepath.Clean(src)] {
		return src
	}
	for n := 2; ; n++ {
		cand := NumberedSrc(src, n)
		if !used[filepath.Clean(cand)] {
			return cand
		}
	}
}
