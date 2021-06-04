package gobgp

import (
	"encoding/binary"
	"fmt"
)

/*
	BGP message header
*/
type header struct {
	Length uint16
	Type   uint8
}

/*
	Encode BGP message header to the wire format
*/
func (h *header) marshal() []byte {
	buf := make([]byte, headerLen)
	for i := 0; i < 16; i++ {
		buf[i] = 0xff
	}
	binary.BigEndian.PutUint16(buf[16:], h.Length)
	buf[18] = h.Type
	return buf
}

/*
	Decode BGP message header from the wire format
*/
func (h *header) unmarshal(buf []byte) (int, error) {
	if len(buf) < headerLen {
		return 0, NewError(1, 2, fmt.Sprintf("unpack: buffer size too small: %d < %d", len(buf), headerLen))
	}
	h.Length = binary.BigEndian.Uint16(buf[16:])
	h.Type = buf[18]
	return headerLen, nil
}
