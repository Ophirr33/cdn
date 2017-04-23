package main

import (
	"bufio"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
)

type host struct {
	loc  latLong
	conn *net.TCPConn
}

type latLong struct {
	lat  float64
	long float64
}

type router struct {
	hosts   map[string]host
	clients map[string]map[string]float64
	mutex   sync.Mutex
}

func (r *router) init(port int) error {
	r.hosts = make(map[string]host)
	r.clients = make(map[string]map[string]float64)
	return r.parseEC2AndConnect(port)
}

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

func (r *router) getServer(ip string) string {
	var servers, exists = r.clients[ip]
	var result = ""
	if !exists || len(servers) == 0 {
		result = r.getClosestServer(ip)
	} else {
		var minRTT = 0.0
		for server, rtt := range servers {
			if minRTT == 0.0 || rtt < minRTT {
				minRTT = rtt
				result = server
			}
		}
	}
	go r.sendPingRequests(ip)
	return result
}

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

func getLatLong(ip string) latLong {
	var lat = 1.0
	var long = 1.0
	return latLong{lat, long}
}

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
		var splitLine = strings.Fields(line)
		if len(splitLine) != 2 {
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
