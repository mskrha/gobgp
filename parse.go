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

	return "", fmt.Errorf("Not found any valid peer IP address")
}

func parseNotificationMessage(m msgNotification) (ret string, err error) {
	switch m.Code {
	case 1:
		ret = fmt.Sprintf("Message Header Error, %s", msgErrSubCodesMsg[m.SubCode])
	case 2:
		ret = fmt.Sprintf("OPEN Message Error, %s", msgErrSubCodesOpen[m.SubCode])
	case 3:
		ret = fmt.Sprintf("UPDATE Message Error, %s", msgErrSubCodesUpdate[m.SubCode])
	case 4:
		ret = fmt.Sprintf("Hold Timer Expired")
	case 5:
		ret = fmt.Sprintf("Finite State Machine Error")
	case 6:
		ret = fmt.Sprintf("Cease, %s", msgErrSubCodesCease[m.SubCode])
	}
	if len(m.Data) > 0 {
		ret = fmt.Sprintf("%s, %s", ret, m.Data)
	}
	return
}
