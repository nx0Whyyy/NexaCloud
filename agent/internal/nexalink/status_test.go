package nexalink

import (
	"bufio"
	"bytes"
	"testing"
)

func TestVarintRoundTrip(t *testing.T) {
	for _, value := range []int{0, 1, 127, 128, 25565, 2097151} {
		decoded, err := readVarint(bufio.NewReader(bytes.NewReader(varint(value))))
		if err != nil || decoded != value {
			t.Fatalf("round trip %d: value=%d error=%v", value, decoded, err)
		}
	}
}
