package main

import (
	"bufio"
	"bytes"
	"testing"
)

func TestMinecraftVarIntRoundTrip(t *testing.T) {
	for _, value := range []int{0, 1, 127, 128, 255, 2147483647, -1} {
		buffer := bytes.NewBuffer(nil)
		writeVarInt(buffer, value)
		got, err := readVarInt(bufio.NewReader(buffer))
		if err != nil || got != value {
			t.Fatalf("round trip %d = %d, err=%v", value, got, err)
		}
	}
}
