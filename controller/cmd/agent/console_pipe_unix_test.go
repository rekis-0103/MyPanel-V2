//go:build unix

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestWriteConsolePipeSendsExactlyOneCommand(t *testing.T) {
	pipePath := filepath.Join(t.TempDir(), "console-in")
	if err := unix.Mkfifo(pipePath, 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := os.OpenFile(pipePath, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := writeConsolePipe(pipePath, "say hello"); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "say hello\n" {
		t.Fatalf("pipe received %q", line)
	}
}

func TestWriteConsolePipeRejectsRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console-in")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeConsolePipe(path, "stop"); err == nil {
		t.Fatal("regular file was accepted as console input")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatalf("regular file changed to %q, err=%v", data, err)
	}
}
