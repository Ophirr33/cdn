package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

type pingServer struct {
	connection *net.TCPConn
}

func (pingServer *pingServer) start() {
	connReader := bufio.NewReader(pingServer.connection)
	for {
		line, err := connReader.ReadString('\n')
		if errorCheck(err) {
			break
		}
		ip := net.ParseIP(strings.Replace(line, "\n", "", -1))
		if ip == nil {
			fmt.Fprintln(os.Stderr, "Could not parse ip address, skipping")
			continue
		}
		ping := exec.Command("ping", ip.String(), "-c", "3", "-l", "3")

		out, err := ping.Output()
		if errorCheck(err) {
			continue
		}
		split0 := bytes.Split(out, []byte("\n"))
		if len(split0) < 2 {
			fmt.Fprintln(os.Stderr, "Could not parse ping lines: ", string(out))
			continue
		}
		split1 := bytes.Fields(split0[len(split0)-2])
		if len(split1) != 7 {
			fmt.Fprintln(os.Stderr, "Could not parse ping output: ", string(out))
			continue
		}
		split2 := bytes.Split(split1[3], []byte("/"))
		if len(split2) != 4 {
			fmt.Fprintln(os.Stderr, "Could not parse ping average: ", string(out))
			continue
		}
		avg := split2[1]
		pingServer.connection.Write(append([]byte(ip.String()+" "), append(avg, '\n')...))
	}
}
