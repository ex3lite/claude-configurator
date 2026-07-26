//go:build !windows

package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var execProcess = syscall.Exec

func launchPrepared(prepared Prepared, currentVersion string, originalArgs []string) error {
	if err := replaceExecutable(prepared.Binary, prepared.Target); err != nil {
		return fmt.Errorf("install update: %w", err)
	}
	_ = os.Remove(prepared.Binary)
	_ = os.Remove(prepared.Helper)
	_ = os.Remove(filepath.Dir(prepared.Helper))

	environment := withEnvironment(os.Environ(), updatedFromEnv, currentVersion)
	environment = withEnvironment(environment, restartModeEnv, "foreground")
	arguments := append([]string{prepared.Target}, originalArgs...)
	return execProcess(prepared.Target, arguments, environment)
}
