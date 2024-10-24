package main

import (
	"bufio"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
)

type Downloader struct {
	urls     []string
	invalids map[string]any
	urlsSet  map[string]any
	result   chan []string
}

type result struct {
	data  []string
	mutex *sync.Mutex
}

func NewDownloader(urls []string, invalids map[string]any) *Downloader {
	return &Downloader{
		urls:     urls,
		invalids: invalids,
		urlsSet:  make(map[string]any),
	}
}

func (d *Downloader) GetHosts() []string {
	hosts := make([]string, 0)
	c := 0
	for i, _ := range d.urlsSet {
		hosts = append(hosts, i)
		c++
	}
	// Sort data
	slices.Sort(hosts)
	return hosts
}

func (d *Downloader) download(from string, r chan []string) {
	log.Println("Downloading", from)
	total := 0
	resp, err := http.Get(from)
	if err != nil {
		log.Println("Error downloading", from, err.Error())
		return
	}
	defer resp.Body.Close()
	result := make([]string, 0)
	reader := bufio.NewReader(resp.Body)
	for {
		s, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		// parse line
		if len(s) > 0 {
			if s[0] == '#' {
				continue
			}
		}
		fields := strings.Fields(s)
		if len(fields) > 1 {
			host := fields[1]
			if host != "" {
				if strings.ContainsAny(host, "|^") {
					continue
				}
				total++
				_, found := d.invalids[host]
				if found {
					continue
				}
				result = append(result, host)
			}
		} else {
			// Host only
			if len(fields) > 0 {
				host := fields[0]
				if host != "" {
					if strings.ContainsAny(host, "|^") {
						continue
					}
					_, found := d.invalids[host]
					if found {
						continue
					}
					result = append(result, fields[0])
				}
			}
		}
	}
	log.Println("finished downloading and parsing", from)
	r <- result
}

func (d *Downloader) Run() {
	var wg sync.WaitGroup
	d.result = make(chan []string, len(d.urls))
	for _, url := range d.urls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.download(url, d.result)
		}()
	}
	wg.Wait()
	log.Println("Postprocessing...")
	stop := false
	for {
		if stop {
			break
		}
		select {
		case r := <-d.result:
			for _, url := range r {
				d.urlsSet[url] = nil
			}
		default:
			stop = true
			break
		}
	}
	log.Println("Finished")
}
