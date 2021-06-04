package gobgp

import (
	"strconv"
)

type Error struct {
	Code    int
	Subcode int
	Err     string
}

func NewError(code, subcode int, extra string) *Error {
	return &Error{code, subcode, extra}
}

func (e *Error) Error() string {
	s := "bgp: "
	if v, ok := errorCodes[e.Code]; ok {
		s += v
	} else {
		s += strconv.Itoa(e.Code)
	}
	s += ": "

	switch e.Code {
	case 1:
		if v, ok := errorSubcodesHeader[e.Subcode]; ok {
			s += v
		}
	case 2:
		if v, ok := errorSubcodesOpen[e.Subcode]; ok {
			s += v
		}
	case 3:
		if v, ok := errorSubcodesUpdate[e.Subcode]; ok {
			s += v
		}
	default:
		s += strconv.Itoa(e.Subcode)
	}
	if len(e.Err) > 0 {
		s += ": " + e.Err
	}
	return s
}

var errBuf = &Error{Err: "buffer size too small"}

var errorCodes = map[int]string{
	1: "message header error",
	2: "OPEN message error",
	3: "UPDATE message error",
	4: "hold timer expired",
	5: "finite state machine error",
	6: "cease",
}

var errorSubcodesHeader = map[int]string{
	1: "connection not synchronized",
	2: "bad message length",
	3: "bad message type",
}

var errorSubcodesOpen = map[int]string{
	1: "unsupported version number",
	2: "bad peer AS",
	3: "bad BGP identifier",
	4: "unsupported optional parameter",
	// 5 deprecated
	6: "unacceptable hold time",
	7: "unsupported capability",
}

var errorSubcodesUpdate = map[int]string{
	1: "malformed attribute list",
	2: "unrecognized well-known attribute",
	3: "missing well-known attribute",
	4: "attribute flags error",
	5: "attribute length error",
	6: "invalid ORIGIN attribute",
	// 7 deprecated
	8:  "invalid NEXT_HOP attribute",
	9:  "optional attribute error",
	10: "invalid network field",
	11: "malformed AS_PATH",
}
