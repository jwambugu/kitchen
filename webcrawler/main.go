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

	err = c.Fetch("https://reddit.com")
	if err != nil {
		log.Fatalln(err)
	}
}
