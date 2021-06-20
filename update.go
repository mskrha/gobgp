package gobgp

import (
	"encoding/binary"
	"fmt"
	"net"
)

/*
	Types of BGP update attributes
*/
const (
	_ = iota
	attributeTypeOrigin
	attributeTypeAsPath
	attributeTypeNextHop
)

/*
	Types of origin
*/
const (
	OriginTypeIGP = iota
	OriginTypeEGP
	OriginTypeIncomplete
)

/*
	Types of AS path
*/
const (
	_ = iota
	AsPathTypeSet
	AsPathTypeSequence
)

/*
	Attribute AS path
*/
type TypeAsPath struct {
	Type uint
	Path []uint16
}

type MsgUpdate struct {
	Withdrawns []string
	Prefixes   []string
	Origin     uint
	AsPath     TypeAsPath
	NextHops   []string
}

func marshalMessageUpdate(m MsgUpdate) (ret []byte, err error) {
	var n uint32
	var mask uint8

	/*
		Withdrawn prefixes
	*/
	bufW := make([]byte, 2)
	if len(m.Withdrawns) > 0 {
		binary.BigEndian.PutUint16(bufW[0:2], uint16(len(m.Withdrawns)*5))
		for _, v := range m.Withdrawns {
			n, mask, err = parsePrefix(v)
			if err != nil {
				return
			}
			buf := make([]byte, 5)
			buf[0] = mask
			binary.BigEndian.PutUint32(buf[1:], n)
			bufW = append(bufW, buf...)
		}
	}

	/*
		Attributes
	*/
	bufA := make([]byte, 2)
	if len(m.Prefixes) > 0 {
		bufOrigin := []byte{0x40, attributeTypeOrigin, 1, byte(m.Origin)}
		bufA = append(bufA, bufOrigin...)

		if len(m.AsPath.Path) == 0 {
			err = fmt.Errorf("Empty AS path")
			return
		}
		bufAsPath := make([]byte, 5)
		bufAsPath[0] = 0x40
		bufAsPath[1] = attributeTypeAsPath
		bufAsPath[3] = byte(m.AsPath.Type)
		bufAsPath[4] = byte(len(m.AsPath.Path))
		a := make([]byte, 2)
		for _, v := range m.AsPath.Path {
			binary.BigEndian.PutUint16(a, v)
			bufAsPath = append(bufAsPath, a...)
		}
		bufAsPath[2] = byte(len(m.AsPath.Path)*2 + 2)
		bufA = append(bufA, bufAsPath...)

		if len(m.NextHops) == 0 {
			err = fmt.Errorf("No next hop defined")
			return
		}
		bufNextHop := make([]byte, 3)
		bufNextHop[0] = 0x40
		bufNextHop[1] = attributeTypeNextHop
		bufNextHop[2] = byte(4 * len(m.NextHops))
		for _, v := range m.NextHops {
			n := net.ParseIP(v).To4()
			if n == nil {
				err = fmt.Errorf("Invalid next hop %s", v)
				return
			}
			bufNextHop = append(bufNextHop, n...)
		}
		bufA = append(bufA, bufNextHop...)

		binary.BigEndian.PutUint16(bufA[0:2], uint16(len(bufOrigin)+len(bufAsPath)+len(bufNextHop)))
	}

	/*
		Announced prefixes
	*/
	var bufNLRI []byte
	for _, v := range m.Prefixes {
		n, mask, err = parsePrefix(v)
		if err != nil {
			return
		}
		buf := make([]byte, 5)
		buf[0] = mask
		binary.BigEndian.PutUint32(buf[1:], n)
		bufNLRI = append(bufNLRI, buf...)
	}

	/*
		Message header
	*/
	ret, err = marshalMessageHeader(msgTypeUpdate, len(bufW)+len(bufA)+len(bufNLRI))
	if err != nil {
		return
	}

	/*
		Put all parts together
	*/
	ret = append(ret, bufW...)
	ret = append(ret, bufA...)
	ret = append(ret, bufNLRI...)

	return
}

func unmarshalMessageUpdate(in []byte) (ret MsgUpdate, err error) {
	/*
		Withdrawn prefixes
	*/
	cntw := binary.BigEndian.Uint16(in[:2])
	if cntw%5 != 0 {
		err = fmt.Errorf("Invalid withdrawn length")
		return
	}
	pos := 2
	var n net.IPNet
	for i := 0; i < int(cntw/5); i++ {
		n.Mask = net.CIDRMask(int(in[pos]), 32)
		n.IP = net.IPv4(in[pos+1], in[pos+2], in[pos+3], in[pos+4])
		ret.Withdrawns = append(ret.Withdrawns, n.String())
		pos += 5
	}

	/*
		Attributes length
	*/
	attrlen := binary.BigEndian.Uint16(in[pos : pos+2])
	if attrlen == 0 {
		return
	}

	/*
		Attributes
	*/
	pos += 2
	attrEnd := pos + int(attrlen)
	for pos < attrEnd {
		// NOT well-known attribute, skipping it
		if in[pos] != 0x40 {
			pos += int(in[pos+2]) + 3
			continue
		}

		switch in[pos+1] {
		case attributeTypeOrigin:
			pos += 3
			ret.Origin = uint(in[pos])
			pos++
		case attributeTypeAsPath:
			pos += 3
			ret.AsPath.Type = uint(in[pos])
			pos++
			aplen := int(in[pos])
			pos++
			var ap uint16
			for i := 0; i < aplen; i++ {
				ap = binary.BigEndian.Uint16(in[pos : pos+2])
				ret.AsPath.Path = append(ret.AsPath.Path, ap)
				pos += 2
			}
		case attributeTypeNextHop:
			pos += 2
			if uint(in[pos])%4 != 0 {
				err = fmt.Errorf("Invalid nexthop attribute length")
				return
			}
			gws := int(in[pos]) / 4
			pos++
			var h string
			for i := 0; i < gws; i++ {
				h = net.IPv4(in[pos], in[pos+1], in[pos+2], in[pos+3]).String()
				ret.NextHops = append(ret.NextHops, h)
				pos += 4
			}
		default:
			pos += int(in[pos])
		}

	}

	/*
		Announced prefixes
	*/
	if (len(in)-pos)%5 != 0 {
		err = fmt.Errorf("Invalid NLRI specification")
		return
	}
	cnt := (len(in) - pos) / 5
	for i := 0; i < cnt; i++ {
		n.Mask = net.CIDRMask(int(in[pos]), 32)
		n.IP = net.IPv4(in[pos+1], in[pos+2], in[pos+3], in[pos+4])
		ret.Prefixes = append(ret.Prefixes, n.String())
		pos += 5
	}

	return
}
