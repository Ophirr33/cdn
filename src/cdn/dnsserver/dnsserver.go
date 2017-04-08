package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type udpPacket struct {
	body []byte
	addr net.UDPAddr
}

// type answer struct {
// 	name: []byte,

func errorCheck(err error) bool {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: ", err)
		return true
	}
	return false
}

func intToString(i int) string {
	return fmt.Sprint(i)
}

func handleRequest(packet *udpPacket) *udpPacket {
	fmt.Println(packet)
	packet.body = []byte("hello")
	return packet
}

func udpRecvSocket(port int, recvPackets chan *udpPacket) {
	var serverAddr, err = net.ResolveUDPAddr("udp", ":"+intToString(port))
	if errorCheck(err) {
		close(recvPackets)
		return
	}
	connection, err := net.ListenUDP("udp", serverAddr)
	if errorCheck(err) {
		close(recvPackets)
		return
	}
	defer connection.Close()

	// using 512 byte buffer as that is the max dns payload size
	var packetBuffer = make([]byte, 512)
	for {
		var length, addr, err = connection.ReadFromUDP(packetBuffer)
		if errorCheck(err) {
			close(recvPackets)
			return
		}
		if length > 0 {
			recvPackets <- &udpPacket{packetBuffer[:length], *addr}
		}
	}
}

func udpSendSocket(port int, sendPackets chan *udpPacket, done chan bool) {
	if port == 65535 {
		port -= 1
	} else {
		port += 1
	}
	var serverAddr, err = net.ResolveUDPAddr("udp", ":"+intToString(port))
	if errorCheck(err) {
		done <- true
		return
	}
	for {
		var packet = <-sendPackets
		var connection, err = net.DialUDP("udp", serverAddr, &(packet.addr))
		if errorCheck(err) {
			done <- true
			return
		}

		_, err = connection.Write(packet.body)
		if errorCheck(err) {
			done <- true
			return
		}

		connection.Close()
	}
}

func dnsServer(port int, name string) {
	var signals = make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// channels for sending and receiving packets
	var sendPackets = make(chan *udpPacket, 1)
	var recvPackets = make(chan *udpPacket, 1)
	// done channel for sending packets
	var done = make(chan bool, 1)

	// starting up udp socket in another thread
	go udpSendSocket(port, sendPackets, done)
	go udpRecvSocket(port, recvPackets)

	for {
		select {
		case sig := <-signals:
			fmt.Println("Caught signal: ", sig)
			return
		case <-done:
			return
		case packet, ok := <-recvPackets:
			if !ok {
				return
			}
			var response = handleRequest(packet)
			if response != nil {
				sendPackets <- response
			}
		default:
			continue
		}
	}
}

func main() {
	defer os.Exit(0)

	// argument parsing, take in -p port and -n name
	var port = flag.Int("p", -1, "Port for dns server to bind on")
	var name = flag.String("n", "", "Base domain name for dns server to serve results for")
	flag.Parse()
	// checking for valid arguments
	if *port == -1 || *name == "" {
		var errMsg string
		if *port == -1 {
			errMsg += "Port number must be provided. "
		}
		if *name == "" {
			errMsg += "Name must be provided as a non-empty string."
		}
		errorCheck(errors.New(errMsg))
	}
	fmt.Println(*port, *name)
	dnsServer(*port, *name)
	fmt.Println("Exiting...")
}
