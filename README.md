[![Go Report Card](https://goreportcard.com/badge/github.com/mskrha/gobgp)](https://goreportcard.com/report/github.com/mskrha/gobgp)

## gobgp

### Description
BGP speaker library in Golang.

### Installation
`go get github.com/mskrha/gobgp`

### Warning
This library implements only the very minimum to be able to establish a BGP session and send updates to the peer.

### Usage
```go
package main

import (
	"fmt"
	"time"

	"github.com/mskrha/gobgp"
)

func main() {
	conf := gobgp.BgpConfig{}
	conf.RouterId = "1.1.1.1"
	conf.ASN = 1111
	conf.HoldTime = 30
	conf.Peer = "2.2.2.2"
	b, err := gobgp.New(conf)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = b.Connect()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = b.Add("1.2.3.4/32")
	if err != nil {
		fmt.Println(err)
	}
	time.Sleep(5 * time.Second)
	err = b.Del("1.2.3.4/32")
	if err != nil {
		fmt.Println(err)
	}

	time.Sleep(15 * time.Second)
	b.Disconnect()
}
```
