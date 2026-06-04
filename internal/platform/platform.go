// Package platform detects the host OS, architecture, and available package manager.
package platform

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// OS values
const (
	OSMacOS = "macos"
	OSLinux = "linux"
	OSWSL   = "wsl"
)

// Package manager values
const (
	PMBrew   = "brew"
	PMAPT    = "apt"
	PMDNF    = "dnf"
	PMPacman = "pacman"
	PMSnap   = "snap"
	PMUnknown = "unknown"
)

// Info describes the host platform.
type Info struct {
	OS            string
	Arch          string
	PackageManager string
}

// Detect returns information about the current host.
func Detect() (Info, error) {
	info := Info{
		OS:   detectOS(),
		Arch: runtime.GOARCH,
	}

	pm, err := detectPackageManager(info.OS)
	if err != nil {
		return info, err
	}
	info.PackageManager = pm

	return info, nil
}

// detectOS returns one ofOSMacOS, OSLinux, OSWSL.
func detectOS() string {
	switch runtime.GOOS {
	case "darwin":
		return OSMacOS
	case "linux":
		if isWSL() {
			return OSWSL
		}
		return OSLinux
	default:
		// unknown — fall through to linux-like
		return OSLinux
	}
}

// isWSL checks for the microsoft kernel string in /proc/version.
func isWSL() bool {
	out, err := exec.Command("cat", "/proc/version").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "microsoft")
}

// detectPackageManager finds the first available package manager for the OS.
func detectPackageManager(os string) (string, error) {
	switch os {
	case OSMacOS:
		if commandExists("brew") {
			return PMBrew, nil
		}
		return PMUnknown, fmt.Errorf("homebrew not found; install from https://brew.sh")
	case OSLinux, OSWSL:
		// order of preference
		for _, pm := range []string{PMAPT, PMDNF, PMPacman, PMSnap} {
			if commandExists(pm) {
				return pm, nil
			}
		}
		return PMUnknown, fmt.Errorf("no supported package manager found (looked for apt, dnf, pacman, snap)")
	default:
		return PMUnknown, fmt.Errorf("unsupported OS: %s", os)
	}
}

// commandExists returns true if the given command is on PATH.
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
