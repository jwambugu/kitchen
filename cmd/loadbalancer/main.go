package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Backend represents a backend server
type Backend struct {
	URL          *url.URL
	Alive        bool
	mu           sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

// SetAlive updates the alive status of a backend
func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	b.Alive = alive
	b.mu.Unlock()
}

// IsAlive checks if the backend is alive
func (b *Backend) IsAlive() (alive bool) {
	b.mu.RLock()
	alive = b.Alive
	b.mu.RUnlock()

	return
}

// LoadBalancer represent a load balancer
type LoadBalancer struct {
	backends []*Backend
	current  uint64
}

// NextBackend returns the next available backend to handle the request
func (lb *LoadBalancer) NextBackend() *Backend {
	next := atomic.AddUint64(&lb.current, uint64(1)%uint64(len(lb.backends)))

	for i := 0; i < len(lb.backends); i++ {
		idx := (int(next) + i) % len(lb.backends)

		if lb.backends[idx].IsAlive() {
			return lb.backends[idx]
		}
	}

	return nil
}

func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second

	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		log.Printf("Site unavailable: %s - %s", u.Host, err.Error())
		return false
	}

	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	return true
}

// HealthCheck pings the backends and updates their status
func (lb *LoadBalancer) HealthCheck() {
	for _, backend := range lb.backends {
		status := isBackendAlive(backend.URL)
		if status {
			log.Printf("Backend %s is alive", backend.URL)
		} else {
			log.Printf("Backend %s is dead", backend.URL)
		}
	}
}

// HealthCheckPeriodically runs a routine health check every interval
func (lb *LoadBalancer) HealthCheckPeriodically(interval time.Duration) {
	t := time.NewTicker(interval)
	for {
		select {
		case <-t.C:
			lb.HealthCheck()
		}
	}
}

// ServeHTTP implements the http.Handler interface for the LoadBalancer
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend := lb.NextBackend()
	if backend == nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	backend.ReverseProxy.ServeHTTP(w, r)
}

func main() {
	port := flag.Int("port", 8080, "Port to serve on")
	flag.Parse()

	var (
		serversUrls = []string{
			"http://localhost:8081",
			"http://localhost:8082",
			"http://localhost:8083",
		}

		lb = LoadBalancer{}
	)

	for _, serverUrl := range serversUrls {
		uri, err := url.Parse(serverUrl)
		if err != nil {
			log.Fatalf("parse url: %s", err.Error())
		}

		proxy := httputil.NewSingleHostReverseProxy(uri)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[%s] Error: %v", serverUrl, err)
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		}

		lb.backends = append(lb.backends, &Backend{
			URL:          uri,
			Alive:        true,
			ReverseProxy: proxy,
		})

		log.Printf("Configured backend: %s\n", serverUrl)
	}

	lb.HealthCheck()

	go lb.HealthCheckPeriodically(time.Minute)

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: &lb,
	}

	log.Printf("Load Balancer started at :%d\n", *port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
