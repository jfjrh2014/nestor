// Package dotfiles deploys dotfiles from a source directory to destinations
// using either a copy or symlink strategy with optional Go template rendering.
package dotfiles

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
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

	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("write: %w", err)}
	}

	return Result{Template: t, Status: StatusDeployed}
}

func (d Deployer) symlink(src, dest string, t Template) Result {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return Result{Template: t, Status: StatusError, Err: fmt.Errorf("mkdir: %w", err)}
	}

	// Try to remove existing dest first.
	// We don't reschedule as "skipped" because the user wants the symlink to reflect src.
	_ = os.Remove(dest)

	if err := os.Symlink(src, dest); err != nil {
		// Peerless fallback: if symlinking fails (permissions, FS, etc.), attempt a unix-style ln.
		if fallbackErr := fallbackCopy(src, dest); fallbackErr != nil {
			return Result{Template: t, Status: StatusError, Err: fmt.Errorf("symlink: %w (fallback: %v)", err, fallbackErr)}
		}
	}

	return Result{Template: t, Status: StatusDeployed}
}

// renderTemplate parses the file as a text/template and runs it with the
// built-in function map (env, home). Variables are not provided by the caller
// yet; Phase 2 will inject secret values.
func renderTemplate(path string) ([]byte, error) {
	tmpl, err := template.New(filepath.Base(path)).
		Funcs(template.FuncMap{
			"env": os.Getenv,
		}).
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

func fallbackCopy(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// Check compares the rendered template source against the deployed dest
// without writing anything. Used by 'nestor diff' for drift detection.
func (d Deployer) Check(src, dest string) CheckStatus {
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
		// For symlinks, check if target points to src
		target, err := os.Readlink(destPath)
		if err != nil || !samePath(target, srcPath) {
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
	}
	return "unknown"
}

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
	if p == "" {
		return p
	}
	if p[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}
