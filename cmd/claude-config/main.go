package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/ex3lite/claude-configurator/internal/config"
	"github.com/ex3lite/claude-configurator/internal/tui"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("claude-config", flag.ContinueOnError)
	flags.SetOutput(stderr)
	scopeName := flags.String("scope", string(config.Global), "initial scope: global, project, or local")
	project := flags.String("project", "", "project path (defaults to current directory)")
	showVersion := flags.Bool("version", false, "print version and exit")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), `Claude Configurator — safe, local Claude Code settings editor

Usage:
  claude-config [--scope global|project|local] [--project PATH]
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
	if _, err := tea.NewProgram(tui.New(workspace, scope, version)).Run(); err != nil {
		fmt.Fprintln(stderr, "TUI error:", err)
		return 1
	}
	return 0
}
