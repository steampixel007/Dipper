//go:build !windows

package main

import (
	"fmt"
	"os"
)

func showStartupError(detail string) {
	fmt.Fprintln(os.Stderr, "Dipper 无法启动："+detail)
}
