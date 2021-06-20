[![Go Report Card](https://goreportcard.com/badge/github.com/mskrha/gobgp)](https://goreportcard.com/report/github.com/mskrha/gobgp)

## gobgp

### Description
Pure Go library implementing the [RFC 4271 - Border Gateway Protocol 4](https://datatracker.ietf.org/doc/html/rfc4271).

### Installation
`go get github.com/mskrha/gobgp`

### Warning
The implementation is not yet full according to the RFC, but most of functionality should be working. All testing was done against the [BIRD 1.6](https://bird.network.cz/).

### Tested (and working) functionality
* Connect to the BGP peer and establish a BGP session
* Re-establish the connection in case of failure
* Send a keepalive packets at 1/3 of holdtime
* Response on notification messages
* Send and receive update messages
* Use internal database of prefixes (modified by the Add and Del functions)
* Resend all prefixes from internal database on reconnect

### Example of usage
```go
package main

import (
	"fmt"
	"time"

	"github.com/mskrha/gobgp"
)

func updateHandler(m gobgp.MsgUpdate) {
	fmt.Printf("%+v\n", m)
}

func main() {
	conf := gobgp.BgpConfig{}
	conf.RouterID = "1.1.1.1"
	conf.ASN = 1111
	conf.HoldTime = 30
	conf.Peer = "2.2.2.2"
	conf.DebugEnabled = true
	b, err := gobgp.New(conf, updateHandler)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = b.Connect()
	if err != nil {
		fmt.Println(err)
		return
	}

	if err := b.Add("12.34.56.78/32", gobgp.OriginTypeIGP, gobgp.TypeAsPath{Type: gobgp.AsPathTypeSequence, Path: []uint16{conf.ASN}}, []string{"1.1.1.1"}); err != nil {
	        fmt.Println(err)
	}

	time.Sleep(5 * time.Second)

	if err := b.Del("12.34.56.78/32"); err != nil {
	        fmt.Println(err)
	}

	time.Sleep(15 * time.Second)
	b.Disconnect()
}
```
