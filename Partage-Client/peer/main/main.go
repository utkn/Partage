package main

import (
	"flag"
	"go.dedis.ch/cs438/peer/impl"
)

func main() {
	port := flag.Uint("port", 8000, "a free port")
	peerID := flag.Uint("id", 1, "peer id must be >= 1")
	introducerAddr := flag.String("i", "", "address of the introducer")
	flag.Parse()
	impl.StartClient(*port, *peerID, *introducerAddr)
}
