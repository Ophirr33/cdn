package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type cache struct {
	memCacheSize         uint
	diskCacheSize        uint
	currentMemCacheSize  uint
	currentDiskCacheSize uint
	memCache             map[string]*http.Response
	diskCache            map[string]string // paths to file names
	built                bool
}

func (cache *cache) init(memCacheSize, diskCacheSize uint) {
	cache.memCacheSize = memCacheSize
	cache.diskCacheSize = diskCacheSize
	cache.currentMemCacheSize = 0
	cache.currentDiskCacheSize = 0
	cache.memCache = make(map[string]*http.Response)
	cache.diskCache = make(map[string]string)
	cache.built = false
}

func (cache *cache) addToCache(path string, resp *http.Response) bool {
	path = strings.ToLower(path)
	if cache.containsPath(path) {
		return false
	}
	return cache.containsPath(path) &&
		(cache.addToMemCache(path, resp) ||
			cache.addToDiskCache(path, resp))
}

func (cache *cache) addToMemCache(path string, resp *http.Response) bool {
	if _, inMem := cache.memCache[path]; resp.ContentLength == -1 || inMem ||
		resp.ContentLength > int64(cache.memCacheSize-cache.currentMemCacheSize) {
		return false
	}
	cache.memCache[path] = resp
	cache.currentMemCacheSize += uint(resp.ContentLength)
	return true
}

func (cache *cache) addToDiskCache(path string, resp *http.Response) bool {
	if _, inDisk := cache.diskCache[path]; resp.ContentLength == -1 || inDisk ||
		resp.ContentLength > int64(cache.diskCacheSize-cache.currentDiskCacheSize) {
		return false
	}
	fileName := strings.Replace(path, "/", "_", -1) + ".txt"
	f, err := os.Create(fileName)
	if errorCheck(err) {
		return false
	}
	defer f.Close()
	err = resp.Write(f)
	if errorCheck(err) {
		return false
	}
	cache.diskCache[path] = fileName
	cache.currentDiskCacheSize += uint(resp.ContentLength)
	return true
}

// containsPath returns whether the path is in the cache, and if it is,
// it returns whether it is in the memory or disk cache
func (cache *cache) containsPath(path string) bool {
	path = strings.ToLower(path)
	if !cache.built {
		return false
	}
	_, inMem := cache.memCache[path]
	_, inDisk := cache.diskCache[path]
	return inMem || inDisk
}

// getFromCache returns the cached http response
func (cache *cache) getFromCache(path string) (*http.Response, error) {
	if !cache.built {
		return nil, errors.New("Cache has not been built yet")
	}
	path = strings.ToLower(path)
	if !cache.containsPath(path) {
		return nil, errors.New("Cache does not contain path `" + path + "`")
	} else if _, inMem := cache.memCache[path]; inMem {
		return cache.memCache[path], nil
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
	fmt.Println("Building cache")
	now := time.Now()
	client := &http.Client{}
	f, err := os.Open(popularFileName)
	if errorCheck(err) {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	mutex := &sync.Mutex{}
	var size uint
	var wg sync.WaitGroup
	for scanner.Scan() {
		if size >= cache.diskCacheSize+cache.memCacheSize {
			break
		}
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			fmt.Println("Making get for ", origin+path)
			resp, err := client.Get(origin + path)
			if !errorCheck(err) {
				mutex.Lock()
				if resp.ContentLength > 0 && resp.ContentLength < int64(cache.diskCacheSize+cache.currentMemCacheSize) {
					size += uint(resp.ContentLength)
				}
				fmt.Println("Added to cache: ",
					cache.addToCache(path, resp),
					" Size of Mem cache (MB): ",
					float64(cache.currentMemCacheSize)/1000000.0,
					" Size of Disk cache (MB): ",
					float64(cache.currentDiskCacheSize)/1000000.0)
				mutex.Unlock()
			}
		}(strings.Fields(scanner.Text())[0])
	}
	wg.Wait()
	cache.built = true // Allow the cache to be used
	elapsed := time.Since(now)
	fmt.Println("Built cache in ", elapsed)
}
