//go:build windows

package selfupdate

import (
	"os"
	"os/exec"
	"strconv"
)

func launchPrepared(prepared Prepared, currentVersion string, originalArgs []string) error {
	arguments := []string{
		helperCommand,
		prepared.Binary,
		prepared.Target,
		strconv.Itoa(os.Getpid()),
		currentVersion,
		"--",
	}
	arguments = append(arguments, originalArgs...)
	command := exec.Command(prepared.Helper, arguments...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Start()
}
