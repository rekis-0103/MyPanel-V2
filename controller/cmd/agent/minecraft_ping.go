package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

type minecraftStatus struct {
	Online    int
	Max       int
	LatencyMS int
}

// pingMinecraft implements the Server List Ping protocol. It is intentionally
// independent from RCON, so player telemetry works even when command access is
// disabled by a runtime or plugin.
func pingMinecraft(ctx context.Context, host string, port int) (minecraftStatus, error) {
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	dialer := net.Dialer{Timeout: time.Second}
	started := time.Now()
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return minecraftStatus{}, err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(time.Second))

	handshake := bytes.NewBuffer(nil)
	writeVarInt(handshake, 0)
	writeVarInt(handshake, -1)
	writeVarInt(handshake, len(host))
	handshake.WriteString(host)
	_ = binary.Write(handshake, binary.BigEndian, uint16(port))
	writeVarInt(handshake, 1)
	if err := writePacket(connection, handshake.Bytes()); err != nil {
		return minecraftStatus{}, err
	}
	if err := writePacket(connection, []byte{0}); err != nil {
		return minecraftStatus{}, err
	}

	reader := bufio.NewReader(connection)
	length, err := readVarInt(reader)
	if err != nil || length < 1 || length > 1<<20 {
		return minecraftStatus{}, errors.New("invalid Minecraft status packet")
	}
	packetID, err := readVarInt(reader)
	if err != nil || packetID != 0 {
		return minecraftStatus{}, errors.New("unexpected Minecraft status packet")
	}
	jsonLength, err := readVarInt(reader)
	if err != nil || jsonLength < 2 || jsonLength > length || jsonLength > 1<<20 {
		return minecraftStatus{}, errors.New("invalid Minecraft status response")
	}
	payload := make([]byte, jsonLength)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return minecraftStatus{}, err
	}
	var response struct {
		Players struct {
			Online int `json:"online"`
			Max    int `json:"max"`
		} `json:"players"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return minecraftStatus{}, fmt.Errorf("decode Minecraft status: %w", err)
	}
	return minecraftStatus{Online: response.Players.Online, Max: response.Players.Max, LatencyMS: int(time.Since(started).Milliseconds())}, nil
}

func writePacket(writer io.Writer, payload []byte) error {
	buffer := bytes.NewBuffer(nil)
	writeVarInt(buffer, len(payload))
	buffer.Write(payload)
	_, err := writer.Write(buffer.Bytes())
	return err
}

func writeVarInt(writer io.ByteWriter, value int) {
	unsigned := uint32(value)
	for {
		current := byte(unsigned & 0x7f)
		unsigned >>= 7
		if unsigned != 0 {
			current |= 0x80
		}
		_ = writer.WriteByte(current)
		if unsigned == 0 {
			return
		}
	}
}

func readVarInt(reader io.ByteReader) (int, error) {
	var value uint32
	for index := 0; index < 5; index++ {
		current, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= uint32(current&0x7f) << (7 * index)
		if current&0x80 == 0 {
			return int(int32(value)), nil
		}
	}
	return 0, errors.New("VarInt exceeds five bytes")
}
