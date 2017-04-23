package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
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
		ip := net.ParseIP(line)
		if ip == nil {
			fmt.Fprintln(os.Stderr, "Could not parse ip address, skipping")
			continue
		}
		ping := exec.Command("ping", ip.String(), "-c", "3", "-l", "3")

		out, err := ping.Output()
		if errorCheck(err) {
			continue
		}
		split1 := bytes.Fields(out)
		if len(split1) != 7 {
			fmt.Fprintln(os.Stderr, "Could not parse ping output: ", string(out))
			continue
		}
		split2 := bytes.Fields(split1[3])
		if len(split2) != 4 {
			fmt.Fprintln(os.Stderr, "Could not parse ping average: ", string(out))
			continue
		}
		avg := split2[1]
		pingServer.connection.Write(append([]byte(ip.String()+" "), append(avg, '\n')...))
	}
}
