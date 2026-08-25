//go:build windows

package main

import "os"

func writeConsolePipe(_ string, _ string) error {
	return os.ErrNotExist
}
