package gobgp

import (
	"encoding/binary"
	"fmt"
	"net"
)

const (
	bgpVersion = 4
)

type msgOpen struct {
	ASN      uint16
	HoldTime uint16
	RouterID string
}

func marshalMessageOpen(m msgOpen) (ret []byte, err error) {
	n := net.ParseIP(m.RouterID).To4()
	if n == nil {
		err = fmt.Errorf("Invalid RouterID")
		return
	}

	buf := make([]byte, 5)

	buf[0] = bgpVersion
	binary.BigEndian.PutUint16(buf[1:3], m.ASN)
	binary.BigEndian.PutUint16(buf[3:5], m.HoldTime)
	buf = append(buf, n...)
	buf = append(buf, 0)

	h, err := marshalMessageHeader(msgTypeOpen, len(buf))
	if err != nil {
		return
	}

	ret = append(ret, h...)
	ret = append(ret, buf...)

	return
}

func unmarshalMessageOpen(in []byte) (ret msgOpen, err error) {
	if in[0] != bgpVersion {
		err = fmt.Errorf("Unsupported BGP protocol version")
		return
	}

	ret.ASN = binary.BigEndian.Uint16(in[1:3])
	ret.HoldTime = binary.BigEndian.Uint16(in[3:5])
	ret.RouterID = net.IPv4(in[5], in[6], in[7], in[8]).String()

	return
}
