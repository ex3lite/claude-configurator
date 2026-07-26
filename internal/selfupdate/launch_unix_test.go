//go:build !windows

package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLaunchReplacesAndExecsInForegroundProcess(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "claude-config")
	binary := filepath.Join(directory, "staged")
	helper := filepath.Join(directory, "helper")
	for path, contents := range map[string]string{
		target: "old",
		binary: "new",
		helper: "old",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	stop := errors.New("exec intercepted")
	originalExec := execProcess
	execProcess = func(path string, arguments, environment []string) error {
		if path != target || !slices.Equal(arguments, []string{target, "--scope", "local"}) {
			t.Fatalf("exec = %q %#v", path, arguments)
		}
		if !slices.Contains(environment, updatedFromEnv+"=0.7.0") ||
			!slices.Contains(environment, restartModeEnv+"=foreground") {
			t.Fatalf("exec environment does not describe the foreground update")
		}
		return stop
	}
	t.Cleanup(func() { execProcess = originalExec })

	err := Launch(Prepared{
		Version: "0.7.1",
		Binary:  binary,
		Helper:  helper,
		Target:  target,
	}, "0.7.0", []string{"--scope", "local"})
	if !errors.Is(err, stop) {
		t.Fatalf("Launch error = %v", err)
	}
	if contents, err := os.ReadFile(target); err != nil || string(contents) != "new" {
		t.Fatalf("updated target = %q, %v", contents, err)
	}
}

func TestNeedsManualRestartForLegacyUnixUpdater(t *testing.T) {
	t.Setenv(updatedFromEnv, "0.7.0")
	t.Setenv(restartModeEnv, "")
	if from, needed := NeedsManualRestart(); !needed || from != "0.7.0" {
		t.Fatalf("NeedsManualRestart = %q, %v", from, needed)
	}
	t.Setenv(restartModeEnv, "foreground")
	if _, needed := NeedsManualRestart(); needed {
		t.Fatal("foreground restart was treated as detached")
	}
}
