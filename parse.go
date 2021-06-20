package gobgp

import (
	"encoding/binary"
	"net"
)

func parsePrefix(x string) (n uint32, m uint8, err error) {
	_, p, err := net.ParseCIDR(x)
	if err != nil {
		return
	}
	n = binary.BigEndian.Uint32(p.IP.To4())
	s, _ := p.Mask.Size()
	m = uint8(s)
	return
}
