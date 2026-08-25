//go:build unix

package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func writeConsolePipe(pipePath, command string) error {
	info, err := os.Lstat(pipePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		return errors.New("managed console input is not a named pipe")
	}
	fd, err := unix.Open(pipePath, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open managed console pipe: %w", err)
	}
	defer unix.Close(fd)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("inspect managed console pipe: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFIFO {
		return errors.New("managed console input changed type")
	}
	payload := []byte(command + "\n")
	written, err := unix.Write(fd, payload)
	if err != nil {
		return fmt.Errorf("write managed console pipe: %w", err)
	}
	if written != len(payload) {
		return errors.New("managed console pipe accepted a partial command")
	}
	return nil
}
