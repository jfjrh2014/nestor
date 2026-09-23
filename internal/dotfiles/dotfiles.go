// Package dotfiles deploys dotfiles from a source directory to destinations
// using either a copy or symlink strategy with optional Go template rendering.
package dotfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jfjrh2014/nestor/internal/fsutil"
	"github.com/jfjrh2014/nestor/internal/pathutil"
)

// Strategy controls how a file is deployed.
type Strategy string

const (
	// StrategyCopy copies the rendered file to dest.
	StrategyCopy Strategy = "copy"
	// StrategySymlink creates a symlink from dest to src.
	StrategySymlink Strategy = "symlink"
)

// Template maps a source template to a destination.
type Template struct {
	Src  string
	Dest string
}

// Result reports the outcome for one template deployment.
type Result struct {
	Template Template
	Status   Status
	Err      error
}

// Status of a deployment.
type Status int

const (
	StatusDeployed Status = iota
	StatusSkipped
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusDeployed:
		return "deployed"
	case StatusSkipped:
		return "skipped"
	case StatusError:
		return "error"
	}
	return "unknown"
}

// Deployer renders and deploys a single template.
//
// Funcs exposes the template functions available inside templates (e.g. env).
type Deployer struct {
	Strategy Strategy
	Source   string // root dir that template Src paths resolve from
}

// DeployAll deploys every template and returns a result per template.
// Template rendering is only applied when the src has a .tmpl extension;
// other files are copied/linked verbatim.
func (d Deployer) DeployAll(temps []Template) []Result {
	out := make([]Result, 0, len(temps))
	for _, t := range temps {
		out = append(out, d.Deploy(t))
	}
	return out
}

// Deploy renders the template at t.Src and writes it to t.Dest.
func (d Deployer) Deploy(t Template) Result {
	if pathutil.IsOtherUserTilde(t.Dest) {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("dest %q uses the ~user/... form, which nestor does not expand (it would deploy into your own home)", t.Dest)}
	}
	src := absPath(d.Source, t.Src)
	dest := expandHome(t.Dest)

	if _, err := os.Stat(src); err != nil {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("src: %w", err)}
	}

	switch d.Strategy {
	case StrategySymlink:
		return d.symlink(src, dest, t)
	default:
		return d.copy(src, dest, t)
	}
}

func (d Deployer) copy(src, dest string, t Template) Result {
	// Never write through a pre-existing symlink at dest: WriteFile follows
	// links, so a chezmoi/stow deployment, an old symlink-strategy leftover,
	// or a hand-made link would have its TARGET silently overwritten — and a
	// dangling link would create a brand-new file wherever it points. Refuse
	// instead; removing or replacing the link is the user's call.
	if info, err := os.Lstat(dest); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("dest %q is a symlink; remove it or switch strategy to symlink before deploying", t.Dest)}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("mkdir: %w", err)}
	}

	// Render template if extension is .tmpl, otherwise plain copy.
	var (
		data []byte
		err  error
	)
	if strings.HasSuffix(src, ".tmpl") {
		data, err = renderTemplate(src)
	} else {
		data, err = os.ReadFile(src)
	}
	if err != nil {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("read: %w", err)}
	}

	// Mode flows from src: a 0600 ssh key deployed at 0644 is a real leak.
	// Perm applies only at creation, so a re-deploy keeps the dest's own
	// chmod — same philosophy as skip-hand-edits: we don't stomp local state.
	perm := os.FileMode(0o644)
	if info, err := os.Stat(src); err == nil {
		perm = info.Mode().Perm()
	}

	// Durable write: temp+fsync+rename (a crash mid-write used to leave half
	// a dotfile; the dest now holds either the old or the new contents). On a
	// redeploy the dest keeps its own chmod — perm still applies at creation
	// only, WritePerm hands back the existing mode for WriteFileSync to apply.
	writePerm, err := fsutil.WritePerm(dest, perm)
	if err != nil {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("write: %w", err)}
	}
	if err := fsutil.WriteFileSync(dest, data, writePerm); err != nil {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("write: %w", err)}
	}

	return Result{Template: t, Status: StatusDeployed}
}

func (d Deployer) symlink(src, dest string, t Template) Result {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("mkdir: %w", err)}
	}

	// Templates must be rendered before linking: a symlink pointed at the raw
	// .tmpl file deploys unrendered {{...}} syntax into the dotfile, bypassing
	// missingkey=error and every template feature (the copy path renders
	// first, so it never had this bug). The rendered bytes live in a stable
	// .nestor-rendered/ file beside the source — refreshed on every deploy —
	// because a temp file deleted at Deploy's return would leave the dest a
	// dangling link. Plain files link to src directly, as before.
	linkTarget := src
	if strings.HasSuffix(src, ".tmpl") {
		data, err := renderTemplate(src)
		if err != nil {
			return Result{Template: t, Status: StatusError, Err: fmt.Errorf("render: %w", err)}
		}
		renderPath := renderedLinkPath(src)
		if err := os.MkdirAll(filepath.Dir(renderPath), 0o700); err != nil {
			return Result{Template: t, Status: StatusError, Err: fmt.Errorf("render: %w", err)}
		}
		if err := fsutil.WriteFileSync(renderPath, data, 0o600); err != nil {
			return Result{Template: t, Status: StatusError, Err: fmt.Errorf("render: %w", err)}
		}
		linkTarget = renderPath
	}

	// Swap the link in without destroying the old dest first: the link is
	// created under a temp name beside dest and renamed over it. Removing
	// dest up front turned every later failure (unrenderable target,
	// read-only dir, failed fallback copy) into silent data loss — the old
	// file was already gone when the error surfaced. If the final rename
	// fails, the old dest is still in place; only the temp link litters.
	tmpLink := dest + ".nestor-tmp-link"
	_ = os.Remove(tmpLink)
	if err := os.Symlink(linkTarget, tmpLink); err != nil {
		// Peerless fallback: if symlinking fails (permissions, FS, etc.),
		// attempt a unix-style ln — dest is still intact here, so a failed
		// fallback leaves the user's file untouched.
		if fallbackErr := fallbackCopy(linkTarget, dest); fallbackErr != nil {
			return Result{Template: t, Status: StatusError, Err: fmt.Errorf("symlink: %w (fallback: %v)", err, fallbackErr)}
		}
		if rmErr := os.Remove(dest); rmErr != nil {
			return Result{Template: t, Status: StatusError, Err: fmt.Errorf("symlink: %w (fallback copy landed; removing old dest failed: %v)", err, rmErr)}
		}
		return Result{Template: t, Status: StatusDeployed}
	}
	if err := os.Rename(tmpLink, dest); err != nil {
		os.Remove(tmpLink)
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("rename %q over %q: %w", tmpLink, dest, err)}
	}

	return Result{Template: t, Status: StatusDeployed}
}

// renderTemplate parses the file as a text/template and runs it with the
// built-in function map (env, home). Variables are not provided by the caller
// yet; Phase 2 will inject secret values.
//
// missingkey=error makes an undefined key fail the render instead of silently
// emitting "<no value>" into the deployed dotfile. It does not affect func
// calls like env or literals, only map/index lookups on missing entries.
func renderTemplate(path string) ([]byte, error) {
	tmpl, err := template.New(filepath.Base(path)).
		Funcs(template.FuncMap{
			"env": os.Getenv,
		}).
		Option("missingkey=error").
		ParseFiles(path)
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, nil); err != nil {
		return nil, err
	}

	return []byte(buf.String()), nil
}

// Render is the exported version of renderTemplate, used by 'nestor edit'
// to preview resolved template output without deploying.
func Render(path string) ([]byte, error) {
	return renderTemplate(path)
}

// fallbackCopy writes src's content to dest durably (temp + fsync + rename)
// with src's permission bits. It runs while dest is still intact, so a failed
// copy leaves the user's existing file untouched.
func fallbackCopy(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	perm := os.FileMode(0o644)
	if info, err := os.Stat(src); err == nil {
		perm = info.Mode().Perm()
	}
	return fsutil.WriteFileSync(dest, data, perm)
}

// Check compares the rendered template source against the deployed dest
// without writing anything. Used by 'nestor diff' for drift detection.
func (d Deployer) Check(src, dest string) CheckStatus {
	if pathutil.IsOtherUserTilde(dest) {
		return CheckUnknown
	}
	srcPath := absPath(d.Source, src)
	destPath := expandHome(dest)

	if _, err := os.Stat(srcPath); err != nil {
		return CheckSrcMissing
	}

	stat, err := os.Lstat(destPath)
	if err != nil {
		return CheckAbsent
	}
	if stat.Mode()&os.ModeSymlink != 0 {
		// For symlinks, check if target points to src — either directly
		// (plain files) or at the deployed render of src (the .tmpl symlink
		// strategy links a temp file holding rendered output, see
		// Deployer.symlink).
		target, err := os.Readlink(destPath)
		if err != nil || !isRenderedLink(target, srcPath) {
			return CheckDrifted
		}
		return CheckPresent
	}

	// For copies, compare content
	var srcData []byte
	if strings.HasSuffix(srcPath, ".tmpl") {
		srcData, err = renderTemplate(srcPath)
	} else {
		srcData, err = os.ReadFile(srcPath)
	}
	if err != nil {
		return CheckDrifted
	}

	destData, err := os.ReadFile(destPath)
	if err != nil {
		return CheckAbsent
	}

	if string(srcData) == string(destData) {
		return CheckPresent
	}
	return CheckDrifted
}

// CheckStatus reports the drift state of a dotfile.
type CheckStatus int

const (
	CheckPresent CheckStatus = iota
	CheckDrifted
	CheckAbsent
	CheckSrcMissing
	CheckUnknown
)

func (s CheckStatus) String() string {
	switch s {
	case CheckPresent:
		return "present"
	case CheckDrifted:
		return "drifted"
	case CheckAbsent:
		return "absent"
	case CheckSrcMissing:
		return "src-missing"
	case CheckUnknown:
		return "unsupported-dest"
	}
	return "unknown"
}

// isRenderedLink reports whether a symlink target counts as "deployed from
// srcPath". Plain files link to srcPath itself. Rendered templates link the
// rendered-output file in renderedLinkMarker (see renderedLinkPath);
// recognition verifies the target sits in that dir and its content still
// matches a fresh render, so tampered or stale files read as drift.
//
// renderedLinkMarker names the directory the symlink strategy writes
// rendered template output to before linking: <srcdir>/.nestor-rendered/<src
// base>. It gives the link a stable target that survives Deploy, and lets
// Check recognize a deployed rendered link by content instead of drift.
const renderedLinkMarker = ".nestor-rendered"

// renderedLinkPath maps a template src to its rendered-output file beside it.
func renderedLinkPath(src string) string {
	return filepath.Join(filepath.Dir(src), renderedLinkMarker, filepath.Base(src))
}

// isRenderedLink reports whether a symlink target counts as "deployed from
// srcPath". Plain files link to srcPath itself. Rendered templates link the
// rendered-output file (renderedLinkPath); recognition verifies the target
// sits in the rendered dir and its content still matches a fresh render, so
// tampered or stale files read as drift.
func isRenderedLink(target, srcPath string) bool {
	if samePath(target, srcPath) {
		return true
	}
	if filepath.Base(filepath.Dir(target)) == renderedLinkMarker {
		data, err := os.ReadFile(target)
		if err != nil {
			return false
		}
		rendered, err := renderTemplate(srcPath)
		if err != nil {
			return false
		}
		return string(data) == string(rendered)
	}
	return false
}

// samePath reports whether two paths point at the same location after
// resolving ~ and relative segments. It does not resolve symlinks.
func samePath(a, b string) bool {
	// Normalize for comparison — handles relative vs absolute
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return absA == absB
}

func absPath(source, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(source, p)
}

func expandHome(p string) string {
	home, _ := os.UserHomeDir()
	return pathutil.ExpandHome(p, home)
}
