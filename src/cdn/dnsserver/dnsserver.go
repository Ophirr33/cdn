package main

import (
	"encoding/binary"
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

/* outgoing answers:
qr = 1
aa = 1
qdcount = 1
ancount = 1
nscount = 0
arcount = 0
copy question from incoming answer
name = domain name
type = A
class = IN
ttl = 0
rdlength = length of data
rdata = ip address

incoming answers:
qr = 1
aa = 0
qdcount = 1
ancount = 0
nscount = 0
arcount = 0
name = domain name
type = A

opcode = 0
tc = 0
rd = 0
ra = 0
z = 0
rcode = 0

*/

type dnsPacket struct {
	id     uint16
	qr     bool
	opcode uint8 // truncate to 4 bits when serializing
	aa     bool
	tc     bool
	rd     bool
	ra     bool
	// 3 empty bits for z
	rcode    uint8 // truncate to 4 bits
	qdcount  uint16
	ancount  uint16
	question *dnsQuestion
	answer   *dnsAnswer
}

type dnsQuestion struct {
	qname  [][]byte
	qtype  uint16
	qclass uint16
}

type dnsAnswer struct {
	aname    []byte
	atype    uint16
	aclass   uint16
	ttl      uint32
	rdlength uint16
	rdata    []byte
}

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

func (packet *dnsPacket) parseDNS(bytes []byte) error {
	if len(bytes) < 12 {
		return errors.New("DNS packet must be at least 12 bytes.")
	} else if bytes[3]&112 != 0 {
		return errors.New("Invalid Z value found in packet. Z must be 0.")
	}

	// parsing header
	packet.id = binary.BigEndian.Uint16(bytes[:2])
	packet.qr = bytes[2]&128 != 0
	packet.opcode = (bytes[2] >> 3) & 15
	packet.aa = bytes[2]&4 != 0
	packet.tc = bytes[2]&2 != 0
	packet.rd = bytes[2]&1 != 0
	packet.ra = bytes[3]&128 != 0
	packet.rcode = bytes[3] & 15
	packet.qdcount = binary.BigEndian.Uint16(bytes[4:6])

	// parsing question
	if packet.qdcount >= 1 {
		packet.question = &dnsQuestion{}
		leftover, err := packet.question.parseDNSQuestion(bytes[12:])
		if err != nil {
			return err
		}
		leftoverLen := len(leftover)
		if leftoverLen > 0 {
			fmt.Fprintln(os.Stderr, "Warning, more than one question in dns response, leftover bytes: ", leftoverLen)
		}
	}

	return nil
}

// Initializes the dnsQuestion, and returns the leftover bytes to read
func (question *dnsQuestion) parseDNSQuestion(bytes []byte) ([]byte, error) {
	bLen := len(bytes)
	currentByte := 0
	question.qname = make([][]byte, 0, 2)
	for currentByte < bLen {
		lengthOfDomain := int(bytes[currentByte])
		if lengthOfDomain == 0 {
			currentByte += 1
			break
		} else if lengthOfDomain+currentByte+1 >= bLen {
			return nil, errors.New("Domain name length is longer than entire packet")
		} else {
			question.qname = append(question.qname, bytes[currentByte+1:currentByte+1+lengthOfDomain])
			currentByte += lengthOfDomain + 1
		}
	}
	if bLen < currentByte+4 {
		return nil, errors.New("No data for QType and QClass")
	}
	question.qtype = binary.BigEndian.Uint16(bytes[currentByte : currentByte+2])
	question.qclass = binary.BigEndian.Uint16(bytes[currentByte+2 : currentByte+4])
	return bytes[currentByte+4:], nil
}

func handleRequest(packet *udpPacket) *udpPacket {
	// fmt.Println(packet)
	var dns = &dnsPacket{}
	var err = dns.parseDNS(packet.body)
	fmt.Println("DNS Header: ", *dns)
	fmt.Println("DNS Question: ", dns.question)
	if errorCheck(err) {
		return nil
	}
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
