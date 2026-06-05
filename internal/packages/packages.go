// Package packages handles package spec parsing, resolution, and installation.
package packages

import (
	"fmt"
	"os/exec"
	"strings"
)

// Spec describes a single package to install.
type Spec struct {
	Raw     string // original spec text from config
	Manager string // brew, apt, dnf, pacman, snap
	Sub     string // e.g., "cask" for homebrew/cask
	Name    string // package name
}

// ParseSpec parses "name", "manager: name", or "manager/sub: name".
// defaultManager is used when no explicit manager is given.
func ParseSpec(raw, defaultManager string) Spec {
	raw = strings.TrimSpace(raw)
	s := Spec{Raw: raw, Manager: defaultManager, Name: raw}

	if idx := strings.Index(raw, ":"); idx >= 0 {
		left := strings.TrimSpace(raw[:idx])
		s.Name = strings.TrimSpace(raw[idx+1:])
		if slash := strings.Index(left, "/"); slash >= 0 {
			s.Manager = left[:slash]
			s.Sub = left[slash+1:]
		} else {
			s.Manager = left
		}
	}
	return s
}

// Resolver merges the common package list with the platform-specific one.
type Resolver struct {
	Common []string
	Lists  map[string][]string // keys: "macos", "linux", "wsl"
}

// Resolve returns the merged, deduplicated list for the given platform.
func (r Resolver) Resolve(platform string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(spec string) {
		s := strings.TrimSpace(spec)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, s := range r.Common {
		add(s)
	}
	if list, ok := r.Lists[platform]; ok {
		for _, s := range list {
			add(s)
		}
	}
	return out
}

// Manager installs packages for a given backend.
type Manager interface {
	IsInstalled(s Spec) (bool, error)
	Install(s Spec) error
}

// NewManager picks the right Manager implementation by name.
func NewManager(name string) (Manager, error) {
	switch name {
	case "brew":
		return brewMgr{}, nil
	case "apt":
		return aptMgr{}, nil
	case "dnf":
		return dnfMgr{}, nil
	case "pacman":
		return pacmanMgr{}, nil
	case "snap":
		return snapMgr{}, nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", name)
	}
}

// Status values reported per Spec after InstallAll runs.
const (
	StatusInstalled        = "installed"
	StatusAlreadyInstalled = "already-installed"
	StatusError            = "error"
)

// Result captures the outcome for one Spec.
type Result struct {
	Spec   Spec
	Status string
	Err    error
}

// InstallAll runs Install for every spec. Managers are instantiated lazily
// and reused. Errors on individual specs do not stop the batch.
func InstallAll(specs []Spec, defaultManager string) []Result {
	results := make([]Result, 0, len(specs))
	mgrs := map[string]Manager{}

	ensure := func(name string) (Manager, error) {
		if m, ok := mgrs[name]; ok {
			return m, nil
		}
		m, err := NewManager(name)
		if err != nil {
			return nil, err
		}
		mgrs[name] = m
		return m, nil
	}

	for _, s := range specs {
		m, err := ensure(s.Manager)
		if err != nil {
			results = append(results, Result{Spec: s, Status: StatusError, Err: err})
			continue
		}

		if installed, _ := m.IsInstalled(s); installed {
			results = append(results, Result{Spec: s, Status: StatusAlreadyInstalled})
			continue
		}

		if err := m.Install(s); err != nil {
			results = append(results, Result{Spec: s, Status: StatusError, Err: err})
			continue
		}
		results = append(results, Result{Spec: s, Status: StatusInstalled})
	}
	return results
}

// === backend implementations ===

type brewMgr struct{}

func (brewMgr) IsInstalled(s Spec) (bool, error) {
	flag := "--formula"
	if s.Sub == "cask" {
		flag = "--cask"
	}
	err := exec.Command("brew", "list", flag, s.Name).Run()
	return err == nil, nil
}

func (brewMgr) Install(s Spec) error {
	args := []string{"install"}
	if s.Sub != "" {
		args = append(args, "--"+s.Sub)
	}
	args = append(args, s.Name)
	return exec.Command("brew", args...).Run()
}

type aptMgr struct{}

func (aptMgr) IsInstalled(s Spec) (bool, error) {
	err := exec.Command("dpkg", "-s", s.Name).Run()
	return err == nil, nil
}

func (aptMgr) Install(s Spec) error {
	return exec.Command("apt-get", "install", "-y", s.Name).Run()
}

type dnfMgr struct{}

func (dnfMgr) IsInstalled(s Spec) (bool, error) {
	err := exec.Command("rpm", "-q", s.Name).Run()
	return err == nil, nil
}

func (dnfMgr) Install(s Spec) error {
	return exec.Command("dnf", "install", "-y", s.Name).Run()
}

type pacmanMgr struct{}

func (pacmanMgr) IsInstalled(s Spec) (bool, error) {
	err := exec.Command("pacman", "-Q", s.Name).Run()
	return err == nil, nil
}

func (pacmanMgr) Install(s Spec) error {
	return exec.Command("pacman", "-S", "--noconfirm", s.Name).Run()
}

type snapMgr struct{}

func (snapMgr) IsInstalled(s Spec) (bool, error) {
	err := exec.Command("snap", "list", s.Name).Run()
	return err == nil, nil
}

func (snapMgr) Install(s Spec) error {
	return exec.Command("snap", "install", s.Name).Run()
}
