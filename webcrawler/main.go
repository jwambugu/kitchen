package main

import (
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

	links := c.Start("https://www.example.com/", 10)
	log.Printf("[*] completed, visited %d page(s)", len(links))
}
