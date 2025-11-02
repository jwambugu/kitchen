package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"kitchen/webcrawler/crawler"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	//httpClient := &http.Client{}
	//c, err := crawler.NewCrawler(httpClient, "")
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//
	//ctx := context.Background()
	//
	//links := c.Start(ctx, "https://withlotus.com/blog", 10)
	//log.Printf("[*] completed, visited %d page(s)", len(links))

	var (
		startURL = flag.String("url", "", "Starting URL to crawl (required)")
		destDir  = flag.String("dir", "storage", "Destination directory for downloaded pages")
		depth    = flag.Int("depth", 3, "Maximum crawl depth")
	)

	flag.Parse()

	if *startURL == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Error: -url flag is required")
		flag.Usage()
		os.Exit(1)
	}

	parsedURL, err := url.Parse(*startURL)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: invalid URL: %v\n", err)
		os.Exit(1)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Error: URL must include scheme and host (e.g., https://example.com)")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived signal: %v. Shutting down gracefully...\n", sig)
		cancel()
	}()

	httpClient := &http.Client{}

	c, err := crawler.NewCrawler(httpClient, *destDir)
	if err != nil {
		log.Fatalf("Failed to create crawler: %v\n", err)
	}

	fmt.Printf("Starting crawl of %s\n", *startURL)
	fmt.Printf("Destination directory: %s\n", *destDir)
	fmt.Printf("Max depth: %d\n", *depth)
	fmt.Println("Press Ctrl-C to stop")
	fmt.Println()

	visitedURLs := c.Start(ctx, *startURL, *depth)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("Crawl complete! Visited %d page(s)\n", len(visitedURLs))
	fmt.Printf("Pages saved to: %s\n", *destDir)
	fmt.Println(strings.Repeat("=", 60))

	if errors.Is(ctx.Err(), context.Canceled) {
		fmt.Println("Crawl was interrupted. Resume by running the same command again.")
		os.Exit(130)
	}

}
