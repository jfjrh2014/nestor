package platform

import (
	"errors"
	"strings"
	"testing"
)

// swapEnv redirects the package's environment lookups for the duration of t.
// Providing a zero map skips restoring that seam (callers that fully replace it).
func swapEnv(t *testing.T, newGoos string, path map[string]bool, procVer string, procErr bool) {
	t.Helper()
	oldGoos, oldLook, oldProc := goos, lookPath, readProcVer
	goos = newGoos
	lookPath = func(name string) (string, error) {
		if path[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	if procErr {
		readProcVer = func() ([]byte, error) { return nil, errors.New("read failed") }
	} else {
		readProcVer = func() ([]byte, error) { return []byte(procVer), nil }
	}
	t.Cleanup(func() { goos, lookPath, readProcVer = oldGoos, oldLook, oldProc })
}

func TestDetectOS(t *testing.T) {
	cases := []struct {
		name, goosIn, procVer, want string
	}{
		{"darwin", "darwin", "", OSMacOS},
		{"linux plain", "linux", "Linux version 6.1", OSLinux},
		{"wsl", "linux", "Linux version 5.15.153.1-microsoft-standard-WSL2", OSWSL},
		{"unknown os falls back to linux", "plan9", "", OSLinux},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			swapEnv(t, tc.goosIn, nil, tc.procVer, false)
			if got := detectOS(); got != tc.want {
				t.Errorf("detectOS(%s) = %q, want %q", tc.goosIn, got, tc.want)
			}
		})
	}
}

func TestIsWSL(t *testing.T) {
	t.Run("microsoft kernel string", func(t *testing.T) {
		swapEnv(t, "linux", nil, "Linux version 5.15-microsoft-standard-WSL2", false)
		if !isWSL() {
			t.Error("expected WSL detection for microsoft kernel")
		}
	})
	t.Run("case insensitive", func(t *testing.T) {
		swapEnv(t, "linux", nil, "Linux version 5.15-MICROSOFT-standard", false)
		if !isWSL() {
			t.Error("expected case-insensitive microsoft match")
		}
	})
	t.Run("plain linux", func(t *testing.T) {
		swapEnv(t, "linux", nil, "Linux version 6.1.0", false)
		if isWSL() {
			t.Error("did not expect WSL on plain linux kernel")
		}
	})
	t.Run("read error treated as not wsl", func(t *testing.T) {
		swapEnv(t, "linux", nil, "", true)
		if isWSL() {
			t.Error("expected read failure to short-circuit to false")
		}
	})
}

func TestDetectPackageManager(t *testing.T) {
	t.Run("macos with brew", func(t *testing.T) {
		swapEnv(t, "darwin", map[string]bool{"brew": true}, "", false)
		pm, err := detectPackageManager(OSMacOS)
		if err != nil || pm != PMBrew {
			t.Errorf("got %q, %v; want brew, nil", pm, err)
		}
	})
	t.Run("macos without brew", func(t *testing.T) {
		swapEnv(t, "darwin", map[string]bool{}, "", false)
		pm, err := detectPackageManager(OSMacOS)
		if err == nil || pm != PMUnknown || !strings.Contains(err.Error(), "https://brew.sh") {
			t.Errorf("got %q, %v; want unknown + brew.sh hint", pm, err)
		}
	})
	for _, tc := range []struct{ name, firstFound, want string }{
		{"apt first on linux", "apt", PMAPT},
		{"dnf when apt missing", "dnf", PMDNF},
		{"pacman only", "pacman", PMPacman},
		{"snap last resort", "snap", PMSnap},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := map[string]bool{}
			path[tc.firstFound] = true
			swapEnv(t, "linux", path, "Linux version 6.1", false)
			pm, err := detectPackageManager(OSLinux)
			if err != nil || pm != tc.want {
				t.Errorf("got %q, %v; want %q, nil", pm, err, tc.want)
			}
		})
	}
	t.Run("no linux package manager", func(t *testing.T) {
		swapEnv(t, "linux", map[string]bool{}, "", false)
		pm, err := detectPackageManager(OSLinux)
		if err == nil || pm != PMUnknown || !strings.Contains(err.Error(), "apt, dnf, pacman, snap") {
			t.Errorf("got %q, %v; want unknown + listing of looked-for managers", pm, err)
		}
	})
	t.Run("unsupported os", func(t *testing.T) {
		swapEnv(t, "js", map[string]bool{}, "", false)
		pm, err := detectPackageManager("js")
		if err == nil || pm != PMUnknown || !strings.Contains(err.Error(), "unsupported OS") {
			t.Errorf("got %q, %v; want unknown + unsupported-OS error", pm, err)
		}
	})
}

func TestCommandExists(t *testing.T) {
	swapEnv(t, "linux", map[string]bool{"sh": true}, "", false)
	if !commandExists("sh") {
		t.Error("expected sh to exist on swapped PATH")
	}
	if commandExists("definitely-not-a-real-command-xyz123") {
		t.Error("expected non-existent command to return false")
	}
}

func TestDetect(t *testing.T) {
	t.Run("happy path linux", func(t *testing.T) {
		swapEnv(t, "linux", map[string]bool{"apt": true}, "Linux version 6.1", false)
		info, err := Detect()
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if info.OS != OSLinux || info.Arch == "" || info.PackageManager != PMAPT {
			t.Errorf("got %+v", info)
		}
	})
	t.Run("returns info even on error", func(t *testing.T) {
		swapEnv(t, "linux", map[string]bool{}, "", false)
		info, err := Detect()
		if err == nil {
			t.Fatal("expected error with no package manager")
		}
		if info.OS != OSLinux || info.PackageManager != "" {
			t.Errorf("partial info should carry OS; got %+v", info)
		}
	})
}
