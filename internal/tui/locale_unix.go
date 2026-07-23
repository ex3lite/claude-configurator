//go:build !darwin && !windows

package tui

func platformLocale() string { return "" }
