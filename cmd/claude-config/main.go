package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ex3lite/claude-configurator/internal/config"
	"github.com/ex3lite/claude-configurator/internal/selfupdate"
	"github.com/ex3lite/claude-configurator/internal/statusline"
	"github.com/ex3lite/claude-configurator/internal/tui"
)

var version = "dev"

func main() {
	selfupdate.CleanupStagedHelper()
	if handled, err := selfupdate.HandleHelper(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "Claude Configurator update failed:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "statusline" {
		return runStatusline(args[1:], os.Stdin, stdout, stderr)
	}
	flags := flag.NewFlagSet("claude-config", flag.ContinueOnError)
	flags.SetOutput(stderr)
	scopeName := flags.String("scope", string(config.Global), "initial scope: global, project, or local")
	project := flags.String("project", "", "project path (defaults to current directory)")
	showVersion := flags.Bool("version", false, "print version and exit")
	noUpdate := flags.Bool("no-update", false, "skip the startup update check")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), `Claude Configurator — safe, local Claude Code settings editor

Usage:
  claude-config [--scope global|project|local] [--project PATH]
  claude-config --no-update
  claude-config statusline [--theme auto|nerd|claude|ansi|mono]
  claude-config --version

Options:`)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", flags.Arg(0))
		flags.Usage()
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}
	scope := config.Scope(*scopeName)
	if scope != config.Global && scope != config.Project && scope != config.Local {
		fmt.Fprintf(stderr, "invalid scope %q: use global, project, or local\n", *scopeName)
		return 2
	}
	if *project != "" {
		info, err := os.Stat(*project)
		if err != nil {
			fmt.Fprintln(stderr, "invalid project path:", err)
			return 2
		}
		if !info.IsDir() {
			fmt.Fprintf(stderr, "invalid project path %q: not a directory\n", *project)
			return 2
		}
	}
	workspace, err := config.LoadWorkspace("", *project)
	if err != nil {
		fmt.Fprintln(stderr, "cannot load Claude Code settings:", err)
		return 1
	}
	model := tui.New(workspace, scope, version)
	if !*noUpdate && selfupdate.Enabled(version) {
		model.EnableUpdates(selfupdate.New(version))
	}
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		fmt.Fprintln(stderr, "TUI error:", err)
		return 1
	}
	if final, ok := finalModel.(*tui.Model); ok {
		if update, ready := final.PreparedUpdate(); ready {
			if err := selfupdate.Launch(update, version, args); err != nil {
				fmt.Fprintln(stderr, "cannot start Claude Configurator update:", err)
				return 1
			}
		}
	}
	return 0
}

func runStatusline(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("claude-config statusline", flag.ContinueOnError)
	flags.SetOutput(stderr)
	theme := flags.String("theme", "auto", "color theme: auto, nerd, claude, ansi, or mono")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), `Render Claude Code status JSON from stdin.

Usage:
  claude-config statusline [--theme auto|nerd|claude|ansi|mono]

Options:`)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected statusline argument: %s\n\n", flags.Arg(0))
		flags.Usage()
		return 2
	}
	if err := statusline.Run(stdin, stdout, *theme, time.Now()); err != nil {
		fmt.Fprintln(stderr, "statusline error:", err)
		return 1
	}
	return 0
}
