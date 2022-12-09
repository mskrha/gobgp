package gobgp

import (
	"encoding/binary"
	"fmt"
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

func parsePeerAddress(x string) (string, error) {
	/*
		Valid IP address, just return
	*/
	if net.ParseIP(x) != nil {
		return x, nil
	}

	/*
		Maybe the Peer address is hostname, try to resolve
	*/
	a, err := net.LookupHost(x)
	if err != nil {
		return "", err
	}

	/*
		Error if the response is empty
	*/
	if len(a) == 0 {
		return "", fmt.Errorf("Failed to resolve the peer address to IP")
	}

	/*
		Find first valid IP address in the response
	*/
	for _, v := range a {
		if net.ParseIP(v) != nil {
			return v, nil
		}
	}

	return
}
