package main

import (
	"context"
	"kitchen/webcrawler/crawler"
	"log"
	"net/http"
)

func main() {
	httpClient := &http.Client{}
	c, err := crawler.NewCrawler(httpClient, "")
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	links := c.Start(ctx, "https://example.com/", 10)
	log.Printf("[*] completed, visited %d page(s)", len(links))
}
