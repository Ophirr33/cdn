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
	aname    [][]byte
	atype    uint16
	aclass   uint16
	ttl      uint32
	rdlength uint16
	rdata    []byte
}

// errorCheck is a convenience function that will print errors to std error if there is one
func errorCheck(err error) bool {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: ", err)
		return true
	}
	return false
}

// intToString doesn't actually save us much typing...
func intToString(i int) string {
	return fmt.Sprint(i)
}

// boolToByte converts a bool to a byte...
func boolToByte(b bool) uint8 {
	if b {
		return 1
	} else {
		return 0
	}
}

// bytearraysToDomain converts the byte arrays format of a domain to its string format
func byteArraysToDomain(b [][]byte) string {
	result := ""
	for i, ch := range b {
		result += string(ch)
		if i+1 < len(b) {
			result += "."
		}
	}
	return result
}

// queryDNSToAnswer initialize a default dns packet that points to california
func (packet *dnsPacket) queryDNSToAnswer(ip net.IP, r router) error {
	var returnIP = net.ParseIP(r.getServer(ip.String())).To4()
	if returnIP == nil {
		return errors.New("Bad IP to return")
	}
	packet.qr = true
	packet.aa = true
	packet.ancount = 1
	packet.answer = &dnsAnswer{
		packet.question.qname,
		1,        // A
		1,        // IN
		0,        // no caching yet
		4,        // one ip address
		returnIP} // california
	return nil
}

// parseDNS parses a DNS packet from an array of bytes
func (packet *dnsPacket) parseDNS(bytes []byte) error {
	fmt.Println("BYTES: ", bytes)
	if len(bytes) < 12 {
		return errors.New("DNS packet must be at least 12 bytes.")
	} else if bytes[3]&112 != 0 {
		// Because some people like to set this apparently...
		errorCheck(errors.New("Invalid Z value found in packet. Z must be 0."))
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

// dnsToBytes serializes the dns packet into a byte array
func (dns *dnsPacket) dnsToBytes() ([]byte, error) {
	result := make([]byte, 12, 512)
	binary.BigEndian.PutUint16(result, dns.id)
	result[2] = (boolToByte(dns.qr) << 7) + (dns.opcode & 15 << 3)
	result[2] += (boolToByte(dns.aa) << 2) + (boolToByte(dns.tc) << 1) + boolToByte(dns.rd)
	result[3] = (boolToByte(dns.ra) << 7) + (dns.rcode & 15)
	binary.BigEndian.PutUint16(result[4:], dns.qdcount)
	binary.BigEndian.PutUint16(result[6:], dns.ancount)
	qbytes, err := dns.question.writeToBytes()
	if errorCheck(err) {
		return nil, err
	}
	abytes, err := dns.answer.writeToBytes()
	if errorCheck(err) {
		return nil, err
	}
	result = append(result, qbytes...)
	result = append(result, abytes...)
	if len(result) > 512 {
		return nil, errors.New("Too much data for dns packet")
	} else {
		return result, nil
	}
}

// nameToBytes turns a byte arrays representation back into its bytes representation
func nameToBytes(name [][]byte) ([]byte, error) {
	result := make([]byte, 0, 512)
	for _, bytes := range name {
		bLen := len(bytes)
		if bLen >= 256 {
			return nil, errors.New("Domain too long to be represented by a single byte")
		}
		result = append(result, byte(bLen))
		result = append(result, bytes...)
	}
	result = append(result, 0)
	return result, nil
}

// bigEndianBytes is a convenience method that returns a uint16 as an array of Big Endian bytes
func bigEndianBytes(u16 uint16) []byte {
	result := make([]byte, 2)
	binary.BigEndian.PutUint16(result, u16)
	return result
}

// writeToBytes serializes a dnsQuestion into byte format
func (question *dnsQuestion) writeToBytes() ([]byte, error) {
	result, err := nameToBytes(question.qname)
	if err != nil {
		return nil, err
	}
	result = append(result, bigEndianBytes(question.qtype)...)
	result = append(result, bigEndianBytes(question.qclass)...)
	return result, nil
}

// writeToBytes serializes a dnsAnswer into byte format
func (answer *dnsAnswer) writeToBytes() ([]byte, error) {
	result, err := nameToBytes(answer.aname)
	if err != nil {
		return nil, err
	}
	result = append(result, bigEndianBytes(answer.atype)...)
	result = append(result, bigEndianBytes(answer.aclass)...)
	bttl := make([]byte, 4)
	binary.BigEndian.PutUint32(bttl, answer.ttl)
	result = append(result, bttl...)
	result = append(result, bigEndianBytes(answer.rdlength)...)
	result = append(result, answer.rdata...)
	return result, nil
}

// handleRequest responds to the incoming udpPacket and returns the proper dns response
func handleRequest(packet *udpPacket, name string, r router) *udpPacket {
	// fmt.Println(packet)
	var dns = &dnsPacket{}
	var err = dns.parseDNS(packet.body)
	fmt.Println("DNS Header: ", *dns)
	fmt.Println("DNS Question: ", dns.question)
	if errorCheck(err) || byteArraysToDomain(dns.question.qname) != name {
		return nil
	}
	fmt.Println("DNS Domain: ", byteArraysToDomain(dns.question.qname))
	err = dns.queryDNSToAnswer(packet.addr.IP, r)
	if errorCheck(err) {
		return nil
	}
	packet.body, err = dns.dnsToBytes()
	if errorCheck(err) {
		return nil
	}
	return packet
}

// udpRecvsocket continuously listens for incoming udpPackets and sends them into the channel
func udpRecvSocket(connection *net.UDPConn, recvPackets chan *udpPacket) {
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

// udpSendSocket continously listens for outgoing udpPackets and sends them through the socket
func udpSendSocket(connection *net.UDPConn, sendPackets chan *udpPacket, done chan bool) {
	for {
		var packet = <-sendPackets
		_, err := connection.WriteToUDP(packet.body, &(packet.addr))
		if errorCheck(err) {
			done <- true
			return
		}
	}
}

// dnsServer starts up a dns server that listens for dns answer queries for name on port port
func dnsServer(port int, name string) {
	var signals = make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// channels for sending and receiving packets
	var sendPackets = make(chan *udpPacket, 1)
	var recvPackets = make(chan *udpPacket, 1)
	// done channel for sending packets
	var done = make(chan bool, 1)

	// starting up udp socket, read and write operations done in other threads
	var connection, err = net.ListenUDP("udp4", &(net.UDPAddr{Port: port}))
	if errorCheck(err) {
		return
	}
	defer connection.Close()
	go udpSendSocket(connection, sendPackets, done)
	go udpRecvSocket(connection, recvPackets)

	var router = router{}
	err = router.init(port)
	if errorCheck(err) {
		return
	}

	for {
		select {
		case sig := <-signals:
			fmt.Println("Caught signal: ", sig)
			return
		case <-done:
			fmt.Println("done caught")
			return
		case packet, ok := <-recvPackets:
			if !ok {
				fmt.Println("Error in listening socket")
				return
			}
			var response = handleRequest(packet, name, router)
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
		if errorCheck(errors.New(errMsg)) {
			return
		}
	}
	fmt.Println(*port, *name)
	dnsServer(*port, *name)
	fmt.Println("Exiting...")
}
