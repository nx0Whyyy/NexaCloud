package nexalink

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/nexastudio/nexacloud/pkg/model"
)

func Status(ctx context.Context, address string) (*model.MinecraftTelemetry, error) {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var port uint16
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		return nil, err
	}
	handshake := append(varint(0), varint(765)...)
	handshake = append(handshake, varint(len(host))...)
	handshake = append(handshake, host...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	handshake = append(handshake, portBytes...)
	handshake = append(handshake, varint(1)...)
	if _, err := conn.Write(append(varint(len(handshake)), handshake...)); err != nil {
		return nil, err
	}
	if _, err := conn.Write([]byte{1, 0}); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	if _, err := readVarint(reader); err != nil {
		return nil, err
	}
	if _, err := readVarint(reader); err != nil {
		return nil, err
	}
	size, err := readVarint(reader)
	if err != nil || size < 0 || size > 1<<20 {
		return nil, fmt.Errorf("invalid minecraft status response")
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	var response struct {
		Players struct {
			Online int `json:"online"`
			Max    int `json:"max"`
		} `json:"players"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}
	return &model.MinecraftTelemetry{Players: response.Players.Online, MaxPlayers: response.Players.Max, ShutdownState: "none"}, nil
}

func varint(value int) []byte {
	var out []byte
	for {
		current := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			current |= 0x80
		}
		out = append(out, current)
		if value == 0 {
			return out
		}
	}
}
func readVarint(reader *bufio.Reader) (int, error) {
	value, position := 0, 0
	for {
		current, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= int(current&0x7f) << position
		if current&0x80 == 0 {
			return value, nil
		}
		position += 7
		if position >= 35 {
			return 0, fmt.Errorf("varint too large")
		}
	}
}
func Enabled(address string) bool { return strings.TrimSpace(address) != "" }
