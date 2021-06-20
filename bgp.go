package gobgp

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

const (
	bgpPort = 179 // Default BGP service TCP port

	/*
		Default date/time format for debug messages
	*/
	defaultDebugTimeFormat = "2006-01-02 15:04:05.000000000"

	processQueueLength = 1000
)

type BgpConfig struct {
	/*
		Router ID in dotted format
	*/
	RouterID string

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
		Enabled / disabled debugging messages
	*/
	DebugEnabled bool

	/*
		Datetime prefix for debug messages
	*/
	DebugTimeFormat string
}

type BGP struct {
	/*
		Router ID
	*/
	id string

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
	db map[string]MsgUpdate

	/*
		Underlying TCP connection
	*/
	conn net.Conn

	/*
		Enabled / disabled debugging messages
	*/
	debugEnabled bool

	/*
		Datetime prefix for debug messages
	*/
	debugTimeFormat string

	/*
		Used for serial processing of received messages
	*/
	ch chan message

	/*
		Application defined function for handling update messages
	*/
	updateHandler func(m MsgUpdate)
}

/*
	Create a new BGP instance
*/
func New(c BgpConfig, uf func(m MsgUpdate)) (*BGP, error) {
	var b BGP

	/*
		Validate Router ID
	*/
	if net.ParseIP(c.RouterID).To4() == nil {
		return &b, fmt.Errorf("New: Invalid Router ID")
	}
	b.id = c.RouterID

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
	b.db = make(map[string]MsgUpdate)

	/*
		Enable / disable debugging messages
	*/
	b.debugEnabled = c.DebugEnabled

	/*
		Set date/time format for debugging messages
	*/
	if len(c.DebugTimeFormat) > 0 {
		// Application specified
		b.debugTimeFormat = c.DebugTimeFormat
	} else {
		// Hardcoded default
		b.debugTimeFormat = defaultDebugTimeFormat
	}

	/*
		Initialise channel for message processor
	*/
	b.ch = make(chan message, processQueueLength)

	/*
		Set the update messages handler function
	*/
	if uf != nil {
		// Application specified
		b.updateHandler = uf
	} else {
		// Hardcoded empty default
		b.updateHandler = func(m MsgUpdate) {}
	}

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
	go b.processReply()
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
	close(b.ch)
	return nil
}

/*
	Add prefix to the internal database and send update to the BGP peer
*/
func (b *BGP) Add(p string, o uint, a TypeAsPath, n []string) error {
	if _, e := b.db[p]; e {
		return fmt.Errorf("Add: Prefix %s alredy exists", p)
	}
	b.debug("Adding prefix %s", p)
	var m MsgUpdate
	m.Prefixes = []string{p}
	m.Origin = o
	m.AsPath = a
	m.NextHops = n
	_, err := marshalMessageUpdate(m)
	if err != nil {
		return err
	}
	b.db[p] = m
	return b.sendUpdate(m)
}

/*
	Delete prefix from the internal database and send update to the BGP peer
*/
func (b *BGP) Del(x string) error {
	m, ok := b.db[x]
	if !ok {
		return fmt.Errorf("Del: Prefix %s not found", x)
	}
	b.debug("Removing prefix %s", x)
	delete(b.db, x)
	m.Withdrawns = m.Prefixes
	m.Prefixes = []string{}
	return b.sendUpdate(m)
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
	msg, err := marshalMessageOpen(msgOpen{ASN: b.as, HoldTime: b.hold, RouterID: b.id})
	if err != nil {
		return
	}

	b.debug("%s: Trying to connect", b.peer)
	b.conn, err = net.Dial("tcp", b.peer)
	if err != nil {
		return
	}
	b.debug("%s: Connected", b.peer)

	b.debug("%s: Sending an OPEN message", b.peer)
	_, err = b.conn.Write(msg)

	return
}

/*
	Close the connection to the BGP peer
*/
func (b *BGP) disconnect() {
	b.debug("%s: Disconnecting", b.peer)
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
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
				for _, v := range b.db {
					if err := b.sendUpdate(v); err != nil {
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
	msg, err := marshalMessageHeader(msgTypeKeepAlive, 0)
	if err != nil {
		fmt.Println("sendKeepalive:", err)
		return
	}
	b.debug("%s: Sending a KEEPALIVE message", b.peer)
	if _, err := b.conn.Write(msg); err != nil {
		fmt.Println("sendKeepalive:", err)
		b.disconnect()
	}
}

/*
	Read messages from the BGP peer
*/
func (b *BGP) readReply() {
	buf := make([]byte, 65536)
	var msg message
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
		if n < headerLength {
			fmt.Println("readReply: Too small packet!")
			continue
		}
		pkts := bytes.Split(buf[:n], headerMarker)
		if len(pkts) < 2 {
			fmt.Println("readReply: Invalid packet")
			continue
		}
		for _, v := range pkts[1:] {
			msg, err = unmarshalMessage(v)
			if err != nil {
				fmt.Println("readReply:", err)
				continue
			}
			b.ch <- msg
		}
	}
}

/*
	Process messages received from the BGP peer
*/
func (b *BGP) processReply() {
	for m := range b.ch {
		switch m.Type {
		case msgTypeOpen:
			b.debug("%s: processReply: Got an OPEN message", b.peer)
			go b.sendKeepalive()
		case msgTypeUpdate:
			b.debug("%s: processReply: Got an UPDATE message", b.peer)
			b.updateHandler(m.Data.(MsgUpdate))
		case msgTypeNotification:
			b.debug("%s: processReply: Got a NOTIFICATION message", b.peer)
			fmt.Println(m.Data.(msgNotification))
			b.disconnect()
		case msgTypeKeepAlive:
			b.debug("%s: processReply: Got a KEEPALIVE message", b.peer)
		default:
			fmt.Printf("%s: processReply: BUG BUG BUG\n", b.peer)
		}
	}
}

/*
	Send UPDATE message to the BGP peer
*/
func (b *BGP) sendUpdate(m MsgUpdate) (err error) {
	if b.conn == nil {
		err = fmt.Errorf("sendUpdate: BGP connection NOT ready!")
		return
	}

	msg, err := marshalMessageUpdate(m)
	if err != nil {
		return
	}

	b.debug("%s: Sending an UPDATE message", b.peer)
	_, err = b.conn.Write(msg)
	return
}

func (b *BGP) debug(f string, a ...interface{}) {
	if b.debugEnabled {
		fmt.Printf(time.Now().Format(b.debugTimeFormat)+": "+f+"\n", a...)
	}
}
