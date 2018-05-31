// Dedicated client for measuring speed (RTT and Latency)

package main

import (
	"flag"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/scionproto/scion/go/lib/snet"
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func printUsage() {
	fmt.Println("\ndedicated_speedclient -s SourceSCIONAddress -d DestinationSCIONAddress")
	fmt.Println("\tProvides speed estimates (RTT and latency) from source to dedicated response desination")
	fmt.Println("\tThe SCION address is specified as ISD-AS,[IP Address]:Port")
	fmt.Println("\tIf source port unspecified, a random available one will be used")
	fmt.Println("\tExample SCION address 1-1,[127.0.0.1]:42002\n")
}

func main() {
	var (
		sourceAddress string
		destinationAddress string

		err    error
		local  *snet.Addr
		remote *snet.Addr

		udpConnection *snet.Conn
	)

	// Fetch arguments from command line
	flag.StringVar(&sourceAddress, "s", "", "Source SCION Address")
	flag.StringVar(&destinationAddress, "d", "", "Destination SCION Address")
	flag.Parse()

	// Create the SCION UDP socket
	if len(sourceAddress) > 0 {
		local, err = snet.AddrFromString(sourceAddress)
		check(err)
	} else {
		printUsage()
		check(fmt.Errorf("Error, source address needs to be specified with -s"))
	}
	if len(destinationAddress) > 0 {
		remote, err = snet.AddrFromString(destinationAddress)
		check(err)
	} else {
		printUsage()
		check(fmt.Errorf("Error, destination address needs to be specified with -d"))
	}

	sciondAddr := fmt.Sprintf("/run/shm/sciond/sd%d-%d.sock", local.IA.I, local.IA.A)
	dispatcherAddr := "/run/shm/dispatcher/default.sock"
	snet.Init(local.IA, sciondAddr, dispatcherAddr)

	udpConnection, err = snet.DialSCION("udp4", local, remote)
	check(err)

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 16)

	seed := rand.NewSource(time.Now().UnixNano())
	// Do 5 iterations so we can use average
	var total int64 = 0
	iters := 0
	num_tries := 0
	for iters < 5 && num_tries < 20 {
		num_tries += 1

		id := rand.New(seed).Uint64()
		n := binary.PutUvarint(sendPacketBuffer, id)
		sendPacketBuffer[n] = 0

		time_sent := time.Now()
		_, err = udpConnection.Write(sendPacketBuffer)
		check(err)

		_, _, err = udpConnection.ReadFrom(receivePacketBuffer)
		time_received := time.Now()
		check(err)

		ret_id, n := binary.Uvarint(receivePacketBuffer)
		if ret_id == id {
			total += (time_received.UnixNano() - time_sent.UnixNano())
			iters += 1
		}
	}

	if iters != 5 {
		check(fmt.Errorf("Error, exceeded maximum number of attempts"))
	}

	var difference float64 = float64(total) / float64(iters)

	fmt.Printf("\nSource: %s\nDestination: %s\n", sourceAddress, destinationAddress);
	fmt.Println("Time estimates:")
	// Print in ms, so divide by 1e6 from nano
	fmt.Printf("\tRTT - %.3fms\n", difference/1e6)
	fmt.Printf("\tLatency - %.3fms\n", difference/2e6)
}
