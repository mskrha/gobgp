package gobgp

import (
	"encoding/binary"
	"fmt"
)

/*
	Types of BGP messages
*/
const (
	_ = iota
	msgTypeOpen
	msgTypeUpdate
	msgTypeNotification
	msgTypeKeepAlive
)

type message struct {
	Type uint
	Data interface{}
}

func marshalMessage(m message) (ret []byte, err error) {
	switch m.Data.(type) {
	case msgOpen:
		if m.Type != msgTypeOpen {
			err = fmt.Errorf("Message type mismatch")
			return
		}
	case MsgUpdate:
		if m.Type != msgTypeUpdate {
			err = fmt.Errorf("Message type mismatch")
			return
		}
	case msgNotification:
		if m.Type != msgTypeNotification {
			err = fmt.Errorf("Message type mismatch")
			return
		}
	}

	switch m.Type {
	case msgTypeOpen:
		ret, err = marshalMessageOpen(m.Data.(msgOpen))
	case msgTypeUpdate:
		ret, err = marshalMessageUpdate(m.Data.(MsgUpdate))
	case msgTypeNotification:
		ret, err = marshalMessageNotification(m.Data.(msgNotification))
	case msgTypeKeepAlive:
		/*
			Nothing to marshal, just set the message type
		*/
		ret, err = marshalMessageHeader(msgTypeKeepAlive, 0)
	default:
		err = fmt.Errorf("Invalid message type %d", m.Type)
	}

	return
}

func unmarshalMessage(in []byte) (ret message, err error) {
	/*
		Message length
	*/
	l := binary.BigEndian.Uint16(in[:2])
	if l < headerLength {
		err = fmt.Errorf("Message too small")
		return
	}
	if l != uint16(len(in)+16) {
		err = fmt.Errorf("Length of the message differs from advertised length")
		return
	}

	/*
		Message type
	*/
	ret.Type = uint(in[2])
	switch ret.Type {
	case msgTypeOpen:
		ret.Data, err = unmarshalMessageOpen(in[3:])
	case msgTypeUpdate:
		ret.Data, err = unmarshalMessageUpdate(in[3:])
	case msgTypeNotification:
		ret.Data, err = unmarshalMessageNotification(in[3:])
	case msgTypeKeepAlive:
		// Nothing to parse, just reset timer
	default:
		err = fmt.Errorf("Unknown message type %d", ret.Type)
	}

	return
}
