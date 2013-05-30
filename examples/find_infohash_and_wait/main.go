// Runs a node on system selected UDP port that attempts to collect 100 peers for
// an infohash, then keeps running as a passive DHT node.
//
// IMPORTANT: if the UDP port is not reachable from the public internet, you
// may see very few results.
//
// To collect 100 peers, it usually has to contact some 10k nodes. This process
// is not instant and should take a minute or two, depending on your network
// connection.
//
//
// There is a builtin web server that can be used to collect debugging stats
// from http://localhost:8711/debug/vars.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"

	l4g "code.google.com/p/log4go"
	"github.com/nhelke/dht"
	"net/http"
)

const (
	httpPortTCP = 8711
	dhtPortUDP  = 0 // 0 to let operating system automatically assign a free port
)

var (
	quit       = make(chan bool)
	interrupt  = make(chan os.Signal)
	cpuprofile = flag.String("cpuprofile", "", "write CPU profile to file")
)

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			l4g.Critical("Unable to create CPU profile file: %v", err)
		} else {
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}
	}

	flag.Parse()
	// Change to l4g.DEBUG to see *lots* of debugging information.
	l4g.AddFilter("stdout", l4g.INFO, l4g.NewConsoleLogWriter())
	if len(flag.Args()) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %v <infohash>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example infohash: d1c5676ae7ac98e8b19f63565905105e3c4c37a2\n")
		flag.PrintDefaults()
		os.Exit(1)
	}
	ih, err := dht.DecodeInfoHash(flag.Args()[0])
	if err != nil {
		l4g.Critical("DecodeInfoHash error: %v\n", err)
		os.Exit(1)
	}

	// This is a hint to the DHT of the minimum number of peers it will try to
	// find for the given node. This is not a reliable limit. In the future this
	// might be moved to "PeersRequest()", so the controlling client can have
	// different targets at different moments or for different infohashes.
	targetNumPeers := 5
	d, err := dht.NewDHTNode(dhtPortUDP, targetNumPeers, false)
	if err != nil {
		l4g.Critical("NewDHTNode error: %v", err)
		os.Exit(1)

	}
	// For debugging.
	go http.ListenAndServe(fmt.Sprintf(":%d", httpPortTCP), nil)

	go d.DoDHT()
	go drainresults(d)

	// Signal handling is necessary so that we exit in a clean state capable
	// of producing an optional CPU profile
	signal.Notify(interrupt, os.Interrupt)

F:
	for {
		select {
		case <-time.After(5 * time.Second):
			// Give the DHT some time to "warm-up" its routing table.
			// TODO Possily create a channel to let the DHT adive us when it is
			// nice and hot
			d.PeersRequest(string(ih), false)
		case <-interrupt:
			break F
		}
	}

	quit <- true
	<-quit
}

// drainresults loops, printing the address of nodes it has found.
func drainresults(n *dht.DHT) {
	l4g.Warn("Note that there are many bad nodes that reply to anything you ask.")
	l4g.Warn("Peers found:")
F:
	for {
		select {
		case r := <-n.PeersRequestResults:
			for _, peers := range r {
				for _, x := range peers {
					l4g.Warn("%v", dht.DecodePeerAddress(x))
				}
			}
		case <-quit:
			break F
		}
	}
	quit <- true
}
