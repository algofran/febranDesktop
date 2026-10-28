//go:build !windows

package main

import (
	"fmt"
	"os"
)

func runningAsService() bool { return false }

func runAsService() int { return 1 }

func runServiceCommand(args []string) int {
	fmt.Fprintln(os.Stderr, "service mode is only supported on Windows")
	return 1
}
