package importer

import (
	"os"
	"os/exec"
	"path/filepath"
)

// execLookPath is a wrapper around exec.LookPath so it can be mocked in tests.
var execLookPath = exec.LookPath

// osUserHomeDir is a wrapper around os.UserHomeDir for mocking in tests.
var osUserHomeDir = os.UserHomeDir

// runCommand runs a shell command and returns its stdout as a string.
// It is a var so tests can stub command execution (same seam as the
// cmdRunner in packages and cmdOutput in secrets).
var runCommand = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}

// expandHome replaces a leading ~ with the home directory.
func expandHome(path string) string {
	if len(path) >= 2 && path[0] == '~' && (path[1] == '/' || path[1] == filepath.Separator) {
		home, err := osUserHomeDir()
		if err == nil {
			return home + path[1:]
		}
	}
	return path
}
