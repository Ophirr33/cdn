package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// errorCheck is a convenience method that will print to standard error if err is an Error
func errorCheck(err error) bool {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: ", err)
		return true
	}
	return false
}

// httpServer takes in the port and the url of the origin server
// It initializes a tcp socket and spawns go routines to handle incoming connections
func httpServer(port int, origin string, cache *cache, cdnAddr net.IP) {
	var signals = make(chan os.Signal, 1)
	var conns = make(chan *net.TCPConn, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})

	go func() {
		for {
			connection, err := listener.AcceptTCP()
			if errorCheck(err) {
				close(signals)
				return
			}
			conns <- connection
		}
	}()

	client := &http.Client{}
	if errorCheck(err) {
		return
	}
	defer listener.Close()
	for {
		select {
		case connection, ok := <-conns:
			if !ok {
				return
			}
			go handleConnection(connection, origin, client, cache, cdnAddr)
		case <-signals:
			listener.Close()
			return
		}
	}
}

// handleConnection sends the incoming http request to the origin server
// In the future it will filter incoming connections through a caching layer
func handleConnection(
	connection *net.TCPConn,
	origin string,
	client *http.Client,
	cache *cache,
	cdnAddr net.IP) {
	defer connection.Close()
	if cdnAddr.Equal(net.ParseIP(connection.LocalAddr().String())) || true { // TODO: REMOVE || TRUE (USED FOR TESTING)
		pingServer := pingServer{connection}
		pingServer.start()
		return
	}
	req, err := http.ReadRequest(bufio.NewReader(connection))
	if errorCheck(err) {
		return
	}
	err = nil
	var resp *http.Response
	path := strings.ToLower(req.RequestURI)
	if cache.containsPath(path) {
		resp, err = cache.getFromCache(path)
		if !errorCheck(err) {
			resp.Write(connection)
			errorCheck(err)
			return
		}
		// If there's an error then we try to grab it from the origin
	}
	resp, err = client.Get(origin + path)
	if errorCheck(err) {
		return
	}
	err = resp.Write(connection)
	errorCheck(err)
}

// resolveCDNAddr gets the ip address of the cdn
func resolveCDNAddr() (net.IP, error) {
	ips, err := net.LookupIP("cs5700cdnproject.ccs.neu.edu")
	if err != nil {
		return nil, err
	} else if len(ips) == 0 {
		return nil, fmt.Errorf("No IPs returned")
	} else {
		return ips[0], nil
	}
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
			errMsg += "Origin URL must be provided as a non-empty string. e.g., http://origin.com:8080"
		}
		if errorCheck(errors.New(errMsg)) {
			return
		}
	}
	var bytesInMegabyte uint = 1000000
	cache := &cache{}
	cache.init(10*bytesInMegabyte, 6*bytesInMegabyte)
	go cache.buildCache(*origin, "popular.txt")
	var cdnAddr net.IP
	for addr, err := resolveCDNAddr(); !errorCheck(err); {
		addr, err = resolveCDNAddr()
	}
	fmt.Println(*port, *origin)
	httpServer(*port, *origin, cache, cdnAddr)
	fmt.Println("Exiting...")
}
