package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := flag.Int("port", 8081, "Port to serve on")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hostname, _ := os.Hostname()
		_, _ = fmt.Fprintf(w, "Backend server on port %d, host: %s, Request path: %s\n", *port, hostname, r.URL.Path)
	})

	log.Printf("Backend started at :%d\n", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil); err != nil {
		log.Fatal(err)
	}
}
