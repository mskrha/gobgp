package gobgp

import (
	"encoding/binary"
	"fmt"
)

const (
	headerLength = 19
)

var headerMarker = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

func marshalMessageHeader(t uint, l int) (ret []byte, err error) {
	if l < 0 {
		err = fmt.Errorf("Invalid message length")
		return
	}

	switch t {
	case msgTypeOpen:
	case msgTypeUpdate:
	case msgTypeNotification:
	case msgTypeKeepAlive:
	default:
		err = fmt.Errorf("Unknown message type %d", t)
		return
	}

	ret = append(ret, headerMarker...)

	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(l+headerLength))
	ret = append(ret, buf...)

	ret = append(ret, byte(t))

	return
}
