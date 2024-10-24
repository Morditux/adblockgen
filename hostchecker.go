package main

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type HostChecker struct {
	hosts      []string
	valid      []string
	invalid    []string
	mvalid     *sync.Mutex
	minvalid   *sync.Mutex
	maxThreads int
	resolvers  *Resolvers
}

type Resolvers struct {
	resolvers []*net.Resolver
	current   int
	mutex     *sync.Mutex
}

func NewResolvers() *Resolvers {
	return &Resolvers{
		resolvers: make([]*net.Resolver, 0),
		current:   0,
		mutex:     &sync.Mutex{},
	}
}

func (r *Resolvers) LookupHost(host string) ([]string, error) {
	r.mutex.Lock()
	i := r.current
	r.mutex.Unlock()
	result, err := r.resolvers[i].LookupHost(context.Background(), host)
	r.mutex.Lock()
	r.current++
	if r.current >= len(r.resolvers) {
		r.current = 0
	}
	r.mutex.Unlock()
	return result, err
}

func (r *Resolvers) Create(resolverAddress string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.resolvers = append(r.resolvers, &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, resolverAddress)
		},
	})

}

func (r *Resolvers) InitResolvers(address []string) {
	for _, server := range address {
		r.Create(server)
	}
}

func NewHostChecker(hosts []string, maxThreads int) *HostChecker {
	hc := &HostChecker{
		hosts:      hosts,
		valid:      make([]string, 0),
		invalid:    make([]string, 0),
		minvalid:   &sync.Mutex{},
		mvalid:     &sync.Mutex{},
		maxThreads: maxThreads,
		resolvers:  NewResolvers(),
	}
	hc.resolvers.Create("8.8.8.8:53")
	hc.resolvers.Create("8.8.4.4:53")
	hc.resolvers.Create("1.0.0.1:53")
	hc.resolvers.Create("1.1.1.1:53")
	return hc
}

func (hc *HostChecker) isValid(url string) bool {
	url = strings.TrimSpace(url)
	_, err := hc.resolvers.LookupHost(url) // net.LookupHost(url)
	if err != nil {
		dnsErr, ok := err.(*net.DNSError)
		if ok {
			if dnsErr.IsTimeout {
				log.Printf("dns time out, removing : %s \n", url)
				return false
			}
			if dnsErr.IsTemporary {
				log.Printf("dns temporary error    : %s\n", url)
				return true
			}
			if dnsErr.IsNotFound {
				log.Printf("not found, removing    : %s\n", url)
				return false
			}
		}
	}
	return true
}

func (hc *HostChecker) Start() {

	work := make(chan string, 1024)
	close := make(chan bool)
	wg := &sync.WaitGroup{}

	// Start go routines

	for i := 0; i < hc.maxThreads; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case url := <-work:
					if hc.isValid(url) {
						hc.mvalid.Lock()
						hc.valid = append(hc.valid, url)
						hc.mvalid.Unlock()
					} else {
						hc.minvalid.Lock()
						hc.invalid = append(hc.invalid, url)
						hc.minvalid.Unlock()
					}
				case <-close:
					wg.Done()
					return
				}
			}

		}()
	}
	// Feed threads with work to do
	for _, url := range hc.hosts {
		work <- url
	}
	close <- true
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
