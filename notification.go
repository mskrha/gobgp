package gobgp

import (
	"fmt"
)

type msgNotification struct {
	Code    uint8
	SubCode uint8
	Data    string
}

/*
	BGP notification error codes and subcodes as defined in RFC 4271, section 6
*/
var (
	msgErrCodes = map[uint8]string{
		1: "Message Header Error",
		2: "OPEN Message Error",
		3: "UPDATE Message Error",
		4: "Hold Timer Expired",
		5: "Finite State Machine Error",
		6: "Cease",
	}

	msgErrSubCodesMsg = map[uint8]string{
		1: "Connection Not Synchronized",
		2: "Bad Message Length",
		3: "Bad Message Type",
	}

	msgErrSubCodesOpen = map[uint8]string{
		1: "Unsupported Version Number",
		2: "Bad Peer AS",
		3: "Bad BGP Identifier",
		4: "Unsupported Optional Parameter",
		6: "Unacceptable Hold Time",
		7: "Unsupported Capability",
	}

	msgErrSubCodesUpdate = map[uint8]string{
		1:  "Malformed Attribute List",
		2:  "Unrecognized Well-known Attribute",
		3:  "Missing Well-known Attribute",
		4:  "Attribute Flags Error",
		5:  "Attribute Length Error",
		6:  "Invalid ORIGIN Attribute",
		8:  "Invalid NEXT_HOP Attribute",
		9:  "Optional Attribute Error",
		10: "Invalid Network Field",
		11: "Malformed AS_PATH",
	}

	msgErrSubCodesCease = map[uint8]string{
		1:  "Maximum Number of Prefixes Reached",
		2:  "Administrative Shutdown",
		3:  "Peer De-configured",
		4:  "Administrative Reset",
		5:  "Connection Rejected",
		6:  "Other Configuration Change",
		7:  "Connection Collision Resolution",
		8:  "Out of Resources",
		9:  "Hard Reset",
		10: "BFD Down",
	}
)

func marshalMessageNotification(m msgNotification) (ret []byte, err error) {
	if _, ok := msgErrCodes[m.Code]; !ok {
		err = fmt.Errorf("Invalid notification error code")
		return
	}

	switch m.Code {
	case 1:
		if _, ok := msgErrSubCodesMsg[m.SubCode]; !ok {
			err = fmt.Errorf("Invalid notification message error subcode")
			return
		}
	case 2:
		if _, ok := msgErrSubCodesOpen[m.SubCode]; !ok {
			err = fmt.Errorf("Invalid notification open error subcode")
			return
		}
	case 3:
		if _, ok := msgErrSubCodesUpdate[m.SubCode]; !ok {
			err = fmt.Errorf("Invalid notification update error subcode")
			return
		}
	}

	buf := make([]byte, 2)
	buf[0] = m.Code
	buf[1] = m.SubCode
	if len(m.Data) > 0 {
		buf = append(buf, []byte(m.Data)...)
	}

	h, err := marshalMessageHeader(msgTypeNotification, len(buf))
	if err != nil {
		return
	}

	ret = append(ret, h...)
	ret = append(ret, buf...)

	return
}

func unmarshalMessageNotification(in []byte) (ret msgNotification, err error) {
	l := len(in)
	if l < 2 {
		err = fmt.Errorf("Message too small")
		return
	}
	if l > 2 {
		ret.Data = string(in[2:])
	}
	ret.Code = in[0]
	ret.SubCode = in[1]

	if _, ok := msgErrCodes[ret.Code]; !ok {
		err = fmt.Errorf("Invalid notification error code")
		return
	}

	switch ret.Code {
	case 1:
		if _, ok := msgErrSubCodesMsg[ret.SubCode]; !ok {
			err = fmt.Errorf("Invalid notification message error subcode")
		}
	case 2:
		if _, ok := msgErrSubCodesOpen[ret.SubCode]; !ok {
			err = fmt.Errorf("Invalid notification open error subcode")
		}
	case 3:
		if _, ok := msgErrSubCodesUpdate[ret.SubCode]; !ok {
			err = fmt.Errorf("Invalid notification update error subcode")
		}
	case 6:
		if _, ok := msgErrSubCodesCease[ret.SubCode]; !ok {
			err = fmt.Errorf("Invalid notification cease error subcode")
		}
	}

	return
}
