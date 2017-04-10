package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
)

func errorCheck(err error) bool {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: ", err)
		return true
	}
	return false
}

func httpServer(port int, origin string) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	client := &http.Client{}
	if errorCheck(err) {
		return
	}
	defer listener.Close()
	for {
		connection, err := listener.AcceptTCP()
		if errorCheck(err) {
			return
		}
		go handleConnection(connection, origin, client)
	}
}

// Borrowed and tweaked from go's source code, as they don't provide an easy way to scan carriage returns
func splitCarriageReturn(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	r := bytes.IndexByte(data, '\r')
	n := bytes.IndexByte(data[r+1:], '\n')
	if r >= 0 && n == 0 {
		return r + 2, data[:r], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func handleConnection(connection *net.TCPConn, origin string, client *http.Client) {
	defer connection.Close()
	req, err := http.ReadRequest(bufio.NewReader(connection))
	if errorCheck(err) {
		return
	}
	resp, err := client.Get(origin + req.URL.RequestURI())
	if errorCheck(err) {
		return
	}
	err = resp.Write(connection)
	errorCheck(err)
}

func main() {
	defer os.Exit(0)

	// argument parsing, take in -p port and -n name
	var port = flag.Int("p", -1, "Port for http server to bind on")
	var origin = flag.String("o", "", "URL for the origin server")
	flag.Parse()
	// checking for valid arguments
	if *port == -1 || *origin == "" {
		var errMsg string
		if *port == -1 {
			errMsg += "Port number must be provided. "
		}
		if *origin == "" {
			errMsg += "Origin URL must be provided as a non-empty string."
		}
		if errorCheck(errors.New(errMsg)) {
			return
		}
	}
	fmt.Println(*port, *origin)
	httpServer(*port, *origin)
	fmt.Println("Exiting...")
}
