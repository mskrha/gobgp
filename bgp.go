package gobgp

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const (
	/*
		Types of BGP messages
	*/
	_ = iota
	MsgOpen
	MsgUpdate
	MsgNotification
	MsgKeepalive
	MsgRouterefresh // See RFC 2918

	headerLen = 19

	MaxSize    = 4096 // Maximum size of a BGP message.
	BGPVersion = 4    // Current defined version of BGP.
	bgpPort    = 179  // Default BGP service TCP port
)

/*
	Internal structure holding one network information
*/
type prefix struct {
	Network uint32
	Mask    uint8
}

type BgpConfig struct {
	/*
		Router ID in dotted format
	*/
	RouterId string

	/*
		Local AS number
	*/
	ASN uint16

	/*
		Hold time in seconds
	*/
	HoldTime uint16

	/*
		IPv4 address of the peer
	*/
	Peer string

	/*
		Enabled / disabled debugging messagess
	*/
	DebugEnabled bool

	/*
		Datetime prefix for debug messagess
	*/
	DebugTimeFormat string
}

type BGP struct {
	/*
		Router ID as integer
	*/
	id uint32

	/*
		Local AS number
	*/
	as uint16

	/*
		Hold time in seconds
	*/
	hold uint16

	/*
		Remote peer address:port
	*/
	peer string

	/*
		Is the connection active and should be reconnected?
	*/
	running bool

	/*
		Internal prefixes database
	*/
	db map[string]prefix

	/*
		Underlying TCP connection
	*/
	conn net.Conn

	/*
		Enabled / disabled debugging messagess
	*/
	debugEnabled bool

	/*
		Datetime prefix for debug messagess
	*/
	debugTimeFormat string
}

/*
	Create a new BGP instance
*/
func New(c BgpConfig) (*BGP, error) {
	var b BGP

	/*
		Validate Router ID
	*/
	id := net.ParseIP(c.RouterId).To4()
	if id == nil {
		return &b, fmt.Errorf("New: Invalid Router ID")
	}
	b.id = binary.BigEndian.Uint32(id)

	/*
		Validate AS number
	*/
	if c.ASN == 0 {
		return &b, fmt.Errorf("New: Invalid AS number")
	}
	b.as = c.ASN

	/*
		Validate hold time
	*/
	if c.HoldTime < 3 {
		return &b, fmt.Errorf("New: Hold time too small")
	}
	b.hold = c.HoldTime

	/*
		Validate peer IP address
	*/
	if net.ParseIP(c.Peer) == nil {
		return &b, fmt.Errorf("New: Invalid peer IP address")
	}
	b.peer = fmt.Sprintf("%s:%d", c.Peer, bgpPort)

	/*
		Initialise internal prefixes database
	*/
	b.db = make(map[string]prefix)

	b.debugEnabled = c.DebugEnabled
	b.debugTimeFormat = c.DebugTimeFormat

	return &b, nil
}

/*
	Start the BGP instance and required goroutines
*/
func (b *BGP) Connect() error {
	if b.running {
		return fmt.Errorf("Connect: Alredy running")
	}
	if err := b.connect(); err != nil {
		return err
	}
	b.running = true
	go b.connection()
	go b.keepalive()
	go b.readReply()
	return nil
}

/*
	Stop the BGP instance
*/
func (b *BGP) Disconnect() error {
	if !b.running {
		return fmt.Errorf("Disconnect: Not running")
	}
	b.running = false
	b.disconnect()
	return nil
}

/*
	Add prefix to the internal database and send update to the BGP peer
*/
func (b *BGP) Add(x string) error {
	if _, e := b.db[x]; e {
		return fmt.Errorf("Add: Prefix %s alredy exists", x)
	}
	p, err := parsePrefix(x)
	if err != nil {
		return err
	}
	b.debug("Adding prefix %s", x)
	b.db[x] = p
	return b.sendUpdate("add", p)
}

/*
	Delete prefix from the internal database and send update to the BGP peer
*/
func (b *BGP) Del(x string) error {
	p, ok := b.db[x]
	if !ok {
		return fmt.Errorf("Del: Prefix %s not found", x)
	}
	b.debug("Removing prefix %s", x)
	delete(b.db, x)
	return b.sendUpdate("del", p)
}

/*
	Check whether the specified prefix is or is not in the internal database
*/
func (b *BGP) Exists(x string) bool {
	_, ok := b.db[x]
	return ok
}

func (b *BGP) EnableDebug() {
	b.debugEnabled = true
}

func (b *BGP) DisableDebug() {
	b.debugEnabled = false
}

func (b *BGP) SetDebugTimeFormat(p string) {
	b.debugTimeFormat = p
}

/*
	Establish the connection to the BGP peer
*/
func (b *BGP) connect() (err error) {
	b.debug("%s: Trying to connect", b.peer)
	b.conn, err = net.Dial("tcp", b.peer)
	if err != nil {
		return
	}
	b.debug("%s: Connected", b.peer)

	buf := make([]byte, 10)

	buf[0] = BGPVersion
	binary.BigEndian.PutUint16(buf[1:], b.as)
	binary.BigEndian.PutUint16(buf[3:], b.hold)
	binary.BigEndian.PutUint32(buf[5:], b.id)

	pbuf := make([]byte, 0)
	buf[9] = uint8(len(pbuf))
	buf = append(buf, pbuf...)

	h := &header{Length: headerLen + uint16(10+len(pbuf)), Type: MsgOpen}

	b.debug("%s: Sending an OPEN message", b.peer)
	_, err = b.conn.Write(append(h.marshal(), buf...))

	return
}

/*
	Close the connection to the BGP peer
*/
func (b *BGP) disconnect() {
	b.debug("%s: Disconnecting", b.peer)
	b.conn.Close()
	b.conn = nil
	b.debug("%s: Disconnected", b.peer)
	return
}

/*
	Periodically check the connection and restart if needed
*/
func (b *BGP) connection() {
	for b.running {
		if b.conn == nil {
			b.debug("%s: Not connected, trying to reconnect", b.peer)
			if err := b.connect(); err != nil {
				fmt.Println("connection:", err)
			} else {
				if len(b.db) > 0 {
					b.debug("%s: Sending all learned prefixes", b.peer)
				}
				for _, p := range b.db {
					if err := b.sendUpdate("add", p); err != nil {
						fmt.Println("connection:", err)
					}
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
}

/*
	Periodically send KEEPALIVE message to the BGP peer at interval 1/3 of HOLDTIME
*/
func (b *BGP) keepalive() {
	t := time.NewTicker(time.Duration(b.hold/3) * time.Second)
	go b.sendKeepalive()
	for range t.C {
		if !b.running {
			t.Stop()
			return
		}
		go b.sendKeepalive()
	}
}

/*
	Send a KEEPALIVE message to the BGP peer
*/
func (b *BGP) sendKeepalive() {
	if b.conn == nil {
		return
	}
	h := &header{Length: headerLen, Type: MsgKeepalive}
	b.debug("%s: Sending a KEEPALIVE message", b.peer)
	if _, err := b.conn.Write(h.marshal()); err != nil {
		fmt.Println("sendKeepalive:", err)
		b.disconnect()
	}
}

/*
	Read messagess from the BGP peer
*/
func (b *BGP) readReply() {
	buf := make([]byte, MaxSize)
	for b.running {
		if b.conn == nil {
			fmt.Println("readReply: BGP connection NOT ready!")
			time.Sleep(time.Second)
			continue
		}
		n, err := b.conn.Read(buf)
		if err != nil {
			fmt.Println("readReply:", err)
			b.disconnect()
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if n < headerLen {
			fmt.Println("readReply: Too small packet!")
			continue
		}
		h := &header{}
		h.unmarshal(buf[:headerLen])
		switch h.Type {
		case MsgOpen:
			b.debug("%s: Got an OPEN message", b.peer)
			go b.sendKeepalive()
		case MsgUpdate:
			b.debug("%s: Got an UPDATE message", b.peer)
		case MsgNotification:
			b.debug("%s: Got a NOTIFICATION message", b.peer)
			b.disconnect()
		case MsgKeepalive:
			b.debug("%s: Got a KEEPALIVE message", b.peer)
		case MsgRouterefresh:
			b.debug("%s: Got a ROUTEREFRESH message", b.peer)
		default:
			fmt.Printf("readReply: BGP message type %d not known!\n", h.Type)
			continue
		}
	}
}

/*
	Send UPDATE message to the BGP peer
*/
func (b *BGP) sendUpdate(t string, p prefix) (err error) {
	if b.conn == nil {
		err = fmt.Errorf("sendUpdate: BGP connection NOT ready!")
		return
	}

	/*
		Prefix to be sent
	*/
	bufNLRI := make([]byte, 5)
	bufNLRI[0] = p.Mask
	binary.BigEndian.PutUint32(bufNLRI[1:], p.Network)

	var data []byte
	switch t {
	case "add":
		bufWitdrawn := make([]byte, 2)
		data = append(data, bufWitdrawn...)

		bufTotalPathAttributeLength := make([]byte, 2)
		bufTotalPathAttributeLength[1] = 20
		data = append(data, bufTotalPathAttributeLength...)

		bufAttributeOrigin := make([]byte, 4)
		bufAttributeOrigin[0] = 0x40 // transitive, well-known, complete
		bufAttributeOrigin[1] = 0x01 // attribute ORIGIN
		bufAttributeOrigin[2] = 0x01 // attribute length
		bufAttributeOrigin[3] = 0x00 // IGP
		data = append(data, bufAttributeOrigin...)

		bufAttributeAsPath := make([]byte, 9)
		bufAttributeAsPath[0] = 0x40 // transitive, well-known, complete
		bufAttributeAsPath[1] = 0x02 // attribute AS_PATH
		bufAttributeAsPath[2] = 0x06 // attribute length
		bufAttributeAsPath[3] = 0x02 // AS_SEQUENCE
		bufAttributeAsPath[4] = 0x01 // sequence length
		binary.BigEndian.PutUint16(bufAttributeAsPath[5:], b.as)
		data = append(data, bufAttributeAsPath...)

		bufAttributeNextHop := make([]byte, 7)
		bufAttributeNextHop[0] = 0x40 // transitive, well-known, complete
		bufAttributeNextHop[1] = 0x03 // attribute NEXT_HOP
		bufAttributeNextHop[2] = 0x04 // attribute length
		binary.BigEndian.PutUint32(bufAttributeNextHop[3:], b.id)
		data = append(data, bufAttributeNextHop...)

		data = append(data, bufNLRI...)
	case "del":
		bufWitdrawn := make([]byte, 2)
		bufWitdrawn[1] = 0x05
		data = append(data, bufWitdrawn...)

		data = append(data, bufNLRI...)

		bufTotalPathAttributeLength := make([]byte, 2)
		data = append(data, bufTotalPathAttributeLength...)
	default:
		err = fmt.Errorf("sendUpdate: BUG BUG BUG")
		return
	}
	head := &header{Length: uint16(headerLen + len(data)), Type: MsgUpdate}
	var buf []byte
	buf = append(buf, head.marshal()...)
	buf = append(buf, data...)

	b.debug("%s: Sending an UPDATE message", b.peer)
	_, err = b.conn.Write(buf)
	return
}

func (b *BGP) debug(f string, a ...interface{}) {
	if b.debugEnabled {
		fmt.Printf(time.Now().Format(b.debugTimeFormat)+": "+f+"\n", a...)
	}
}

/*
	Parse CIDR prefix from string to the struct prefix
*/
func parsePrefix(x string) (ret prefix, err error) {
	_, p, err := net.ParseCIDR(x)
	if err != nil {
		return
	}
	ret.Network = binary.BigEndian.Uint32(p.IP.To4())
	s, _ := p.Mask.Size()
	ret.Mask = uint8(s)
	return
}
