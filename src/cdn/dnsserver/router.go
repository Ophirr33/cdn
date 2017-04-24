package main

import (
	"bufio"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const ipSqlCommand string = "\"SELECT latitude, longitude FROM locations JOIN blocks on blocks.locId = locations.locId WHERE %d BETWEEN blocks.startIpNum AND blocks.endIpNum LIMIT 1;\""
const dbName string = "locaiton.db"

// contains lat long of the host as well as the persistent TCP connection
type host struct {
	loc  latLong
	conn *net.TCPConn
}

// represents a latitude longitude pair
type latLong struct {
	lat  float64
	long float64
}

// routing object for routing a client to an ec2 host
type router struct {
	hosts   map[string]host               // host ips to host structs
	clients map[string]map[string]float64 // client ips to host ips to weighted rtts
	mutex   sync.Mutex                    // mutex lock for clients map
}

// initializes the router, given port should be the port ec2 http servers listen on
func (r *router) init(port int) error {
	r.hosts = make(map[string]host)
	r.clients = make(map[string]map[string]float64)
	return r.parseEC2AndConnect(port)
}

// parses the ec2-hosts.txt file
// attempts to establish tcp connections with each host
// starts up threads for reading from connections
func (r *router) parseEC2AndConnect(port int) error {
	file, err := os.Open("ec2-hosts.txt")
	if err != nil {
		return err
	}
	defer file.Close()
	var scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		var text = scanner.Text()
		if strings.Contains(text, "Origin") {
			continue
		}
		var line = strings.Split(text, "\t")
		var url = strings.Split(line[0], "-")
		var ip = strings.Join([]string{url[1], url[2], url[3], strings.Split(url[4], ".")[0]}, ".")
		var conn, err = net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP(ip), Port: port})
		if err != nil {
			return err
		}
		r.hosts[ip] = host{getLatLong(ip), conn}
		go r.getPingResponses(ip)
	}
	return nil
}

// gets the server ip to respond with for the given client ip
func (r *router) getServer(ip string) string {
	var servers, exists = r.clients[ip]
	var result = ""
	// if the client has never been seen before return closest server
	if !exists || len(servers) == 0 {
		result = r.getClosestServer(ip)
	} else {
		// otherwise return server with minimum weighted average rtt for client
		var minRTT = 0.0
		for server, rtt := range servers {
			if minRTT == 0.0 || rtt < minRTT {
				minRTT = rtt
				result = server
			}
		}
	}
	// send out ping requests for client in another thread
	go r.sendPingRequests(ip)
	return result
}

// gets the closest server for the given client ip
func (r *router) getClosestServer(ip string) string {
	var loc = getLatLong(ip)
	var minDistance = 0.0
	var closest = ""
	for ip, host := range r.hosts {
		var dist = distance(loc, host.loc)
		if minDistance == 0.0 || dist < minDistance {
			minDistance = dist
			closest = ip
		}
	}
	return closest
}

// uses the haversine formula to determine distance between two lat-long points
func distance(a, b latLong) float64 {
	aLatRad := a.lat * math.Pi / 180
	aLongRad := a.long * math.Pi / 180
	bLatRad := b.lat * math.Pi / 180
	bLongRad := b.long * math.Pi / 180
	radius := 6378137.0 // radius of the Earth in meters (hopefully Wikipedia is not wrong)
	h := haversine(bLatRad-aLatRad) + (math.Cos(aLatRad) * math.Cos(bLatRad) * haversine(bLongRad-aLongRad))
	return 2.0 * radius * math.Asin(math.Sqrt(h))
}

func haversine(diff float64) float64 {
	return math.Pow(math.Sin(diff/2), 2)
}

func ipStringToInt(ipstr string) int {
	if ipstr == "" {
		return 0
	}
	parts := strings.Split(ipstr, ".")
	first, _ := strconv.ParseInt(parts[0], 10, 32)
	second, _ := strconv.ParseInt(parts[1], 10, 32)
	third, _ := strconv.ParseInt(parts[2], 10, 32)
	fourth, _ := strconv.ParseInt(parts[3], 10, 32)
	return int((first << 24) + (second << 16) + (third << 8) + fourth)
}

// gets the latitude and longitude for the given ip using external database
func getLatLong(ip string) latLong {
	sqlite3 := exec.Command("sqlite3", dbName, fmt.Sprintf(ipSqlCommand, ipStringToInt(ip)))
	out, err := sqlite3.Output()
	if errorCheck(err) {
		return latLong{0.0, 0.0}
	}
	latLongFields := strings.Split(string(out), "|")
	lat, _ := strconv.ParseFloat(latLongFields[0], 64)
	long, _ := strconv.ParseFloat(latLongFields[1], 64)
	return latLong{lat, long}
}

// sends out requests for all ec2 hosts to ping the given ip
func (r *router) sendPingRequests(ip string) {
	var toSend = []byte(ip + "\n")
	for _, host := range r.hosts {
		host.conn.Write(toSend)
	}
}

func (r *router) getPingResponses(ip string) {
	var conn = r.hosts[ip].conn
	var connReader = bufio.NewReader(conn)
	for {
		line, err := connReader.ReadString('\n')
		if errorCheck(err) {
			break
		}
		var splitLine = strings.Fields(strings.Replace(line, "\n", "", -1))
		if len(splitLine) != 2 {
			fmt.Fprintln(os.Stderr, "Could not parse ping response for http server: ", ip)
			continue
		}
		var clientIP = splitLine[0]
		rtt, err := strconv.ParseFloat(splitLine[1], 64)
		if errorCheck(err) {
			continue
		}

		r.mutex.Lock()
		if _, in := r.clients[clientIP]; !in {
			r.clients[clientIP] = make(map[string]float64)
		}
		if avgRTT, in := r.clients[clientIP][ip]; in {
			r.clients[clientIP][ip] = (avgRTT + rtt) / 2.0
		} else {
			r.clients[clientIP][ip] = rtt
		}
		r.mutex.Unlock()
	}
}
