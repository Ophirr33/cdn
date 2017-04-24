package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

type cache struct {
	memCacheSize         uint
	diskCacheSize        uint
	currentMemCacheSize  uint
	currentDiskCacheSize uint
	memCache             map[string][]byte
	diskCache            map[string]string // paths to file names
	built                bool
}

func (cache *cache) init(memCacheSize, diskCacheSize uint) {
	cache.memCacheSize = memCacheSize
	cache.diskCacheSize = diskCacheSize
	cache.currentMemCacheSize = 0
	cache.currentDiskCacheSize = 0
	cache.memCache = make(map[string][]byte)
	cache.diskCache = make(map[string]string)
	cache.built = false
}

func (cache *cache) addToCache(path string, resp *http.Response) bool {
	path = strings.ToLower(path)
	buffer := &bytes.Buffer{}
	err := resp.Write(buffer)
	if errorCheck(err) {
		return false
	}
	copyBuffer := make([]byte, buffer.Len())
	n, err := buffer.Read(copyBuffer)
	if errorCheck(err) || n != len(copyBuffer) {
		return false
	}
	return !cache.containsPath(path) &&
		(cache.addToMemCache(path, copyBuffer) ||
			cache.addToDiskCache(path, copyBuffer))
}

func (cache *cache) addToMemCache(path string, resp []byte) bool {
	if _, inMem := cache.memCache[path]; inMem || len(resp) > int(cache.memCacheSize-cache.currentMemCacheSize) {
		return false
	}
	cache.memCache[path] = resp
	cache.currentMemCacheSize += uint(len(resp))
	return true
}

func (cache *cache) addToDiskCache(path string, resp []byte) bool {
	respLength := len(resp)
	if _, inDisk := cache.diskCache[path]; inDisk ||
		respLength > int(cache.diskCacheSize-cache.currentDiskCacheSize) {
		return false
	}
	fileName := ".cache/" + strings.Replace(path, "/", "_", -1) + ".cache"
	f, err := os.Create(fileName)
	if errorCheck(err) {
		return false
	}
	n, err := f.Write(resp)
	if errorCheck(err) {
		fmt.Fprintln(os.Stderr, n, respLength)
		fmt.Fprintln(os.Stderr, "Failed to write response for path ", path, " to file ", fileName)
		f.Close()
		os.Remove(fileName)
		return false
	}
	defer f.Close()
	cache.diskCache[path] = fileName
	cache.currentDiskCacheSize += uint(respLength)
	return true
}

// containsPath returns whether the path is in the cache, and if it is,
// it returns whether it is in the memory or disk cache
func (cache *cache) containsPath(path string) bool {
	path = strings.ToLower(path)
	_, inMem := cache.memCache[path]
	_, inDisk := cache.diskCache[path]
	return inMem || inDisk
}

// getFromCache returns the cached http response
func (cache *cache) getFromCache(path string) (*http.Response, error) {
	path = strings.ToLower(path)
	if !cache.containsPath(path) {
		return nil, errors.New("Cache does not contain path `" + path + "`")
	} else if _, inMem := cache.memCache[path]; inMem {
		rawBytes := cache.memCache[path]
		resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(rawBytes)), nil)
		if err != nil {
			return nil, err
		} else {
			return resp, nil
		}
	}
	fileName := cache.diskCache[path]
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	resp, err := http.ReadResponse(bufio.NewReader(f), nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (cache *cache) buildCache(origin, popularFileName string) {
	f, err := os.Open(popularFileName)
	if errorCheck(err) {
		return
	}
	defer f.Close()
	getPool := make(chan string)
	fullPool := make(chan bool)

	go func() {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			path := strings.Fields(scanner.Text())[0]
			if strings.HasPrefix(path, "/wiki") {
				getPool <- path
			}
			totalCapacity := cache.diskCacheSize + cache.memCacheSize
			totalSize := cache.currentDiskCacheSize + cache.currentMemCacheSize
			if totalCapacity-totalSize <= 100000 { //.1 MB
				close(fullPool)
				break
			}
		}
	}()

	window := 5 // Number of parallel GETs
	mutex := sync.Mutex{}
	client := http.Client{}
	var wg sync.WaitGroup
	wg.Add(window)
	for i := 0; i < window; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case path := <-getPool:
					resp, err := client.Get(origin + path)
					if !errorCheck(err) && resp.StatusCode >= 200 && resp.StatusCode < 300 {
						mutex.Lock()
						if cache.addToCache(path, resp) {
							fmt.Println("Added", path, "to cache")
						}
						mutex.Unlock()
					}
				case _, ok := <-fullPool:
					if !ok {
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	cache.built = true // Allow the cache to be used
	fmt.Println("Cache finished building")
}
