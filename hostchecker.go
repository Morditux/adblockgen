package main

import (
	"log"
	"net"
	"strings"
	"sync"
)

type HostChecker struct {
	hosts      []string
	valid      []string
	invalid    []string
	mvalid     *sync.Mutex
	minvalid   *sync.Mutex
	bucketSize int
}

func isValid(url string) bool {
	url = strings.TrimSpace(url)
	_, err := net.LookupHost(url)
	if err != nil {
		dnsErr, ok := err.(*net.DNSError)
		if ok {
			if dnsErr.IsTimeout {
				log.Printf("%s dns time out, removing\n", url)
				return false
			}
			if dnsErr.IsTemporary {
				log.Printf("%s dns temporary error, removing\n", url)
			}
			if dnsErr.IsNotFound {
				log.Printf("%s not found, removing\n", url)
				return false
			}
		}
	}
	return true
}

func NewHostChecker(hosts []string, bucketSize int) *HostChecker {
	return &HostChecker{
		hosts:      hosts,
		valid:      make([]string, 0),
		invalid:    make([]string, 0),
		minvalid:   &sync.Mutex{},
		mvalid:     &sync.Mutex{},
		bucketSize: bucketSize,
	}
}

func (hc *HostChecker) Start() {
	wg := new(sync.WaitGroup)
	pos := 0
	for {
		if pos >= len(hc.hosts) {
			break
		}
		bucket := make([]string, 0)
		for counter := 0; counter < hc.bucketSize; counter++ {
			bucket = append(bucket, hc.hosts[pos])
			pos++
			if pos >= len(hc.hosts) {
				break
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < len(bucket); i++ {
				if isValid(bucket[i]) {
					hc.mvalid.Lock()
					hc.valid = append(hc.valid, bucket[i])
					hc.mvalid.Unlock()
				} else {
					hc.minvalid.Lock()
					hc.invalid = append(hc.invalid, bucket[i])
					hc.minvalid.Unlock()
				}
			}
		}()
	}
	wg.Wait()
}

func (hc *HostChecker) Valids() []string {
	hc.mvalid.Lock()
	defer hc.mvalid.Unlock()
	return hc.valid
}

func (hc *HostChecker) Invalids() []string {
	hc.minvalid.Lock()
	defer hc.minvalid.Unlock()
	return hc.invalid
}
