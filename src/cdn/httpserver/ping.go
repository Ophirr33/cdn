package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

type pingServer struct {
	connection *net.TCPConn
}

func (ping *pingServer) start() {
	connReader := bufio.NewReader(ping.connection)
	for {
		line, err := connReader.ReadString('\n')
		if errorCheck(err) {
			break
		}
		fmt.Println(line)
		ip := net.ParseIP(strings.Replace(line, "\n", "", -1))
		if ip == nil {
			fmt.Fprintln(os.Stderr, "Could not parse ip address, skipping")
			continue
		}
		out, err := exec.Command(
			"ping " + ip.String() + " -c 3 -l 3 | tail -1 | awk -F '/' '{print $5}'",
		).Output()
		if errorCheck(err) {
			continue
		}
		fmt.Println(out, "should be on the same line")
		ping.connection.Write(append([]byte(ip.String()+" "), append(out, '\n')...))
	}
}
