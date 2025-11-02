package crawler

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

// alphanumericRegex is a regular expression to match non-alphanumeric characters.
var alphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// DestinationDir is the directory where fetched pages will be saved if none is provided.
const DestinationDir = "storage"

// ErrPageNotFound is returned when a page is not found.
var ErrPageNotFound = errors.New("page not found")

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Crawler struct {
	mu             sync.RWMutex
	httpClient     HttpClient
	destinationDir string
	visitedPages   map[string]struct{}
}

// DownloadAndSave downloads and saves the uri to the provided filename
func (c *Crawler) DownloadAndSave(uri string, filename string) (*bytes.Buffer, error) {
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		file, err := os.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("create file: %w", err)
		}

		defer func(file *os.File) {
			_ = file.Close()
		}(file)

		var buffer bytes.Buffer
		writer := io.MultiWriter(file, &buffer)

		if _, err := io.Copy(writer, resp.Body); err != nil {
			return nil, fmt.Errorf("copy response to file: %w", err)
		}

		// Seek to the beginning of the file for reading
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek file: %w", err)
		}

		return &buffer, nil
	case http.StatusNotFound:
		return nil, ErrPageNotFound
	}

	return nil, fmt.Errorf("request failed with status: %d", resp.StatusCode)
}

func (c *Crawler) FindLinks(uri *url.URL, reader io.Reader) []string {
	tokenizer := html.NewTokenizer(reader)
	foundLinks := make(map[string]struct{})

	for {
		switch tt := tokenizer.Next(); tt {
		case html.ErrorToken:
			links := make([]string, 0, len(foundLinks))

			delete(foundLinks, uri.String())

			for link := range foundLinks {
				links = append(links, link)
			}
			return links

		case html.StartTagToken:
			token := tokenizer.Token()
			if token.DataAtom != atom.A {
				continue
			}

			for _, attr := range token.Attr {
				if attr.Key != "href" {
					continue
				}

				rawUrl := strings.TrimSpace(attr.Val)
				if rawUrl == "" || strings.HasPrefix(rawUrl, "mailto:") || strings.HasPrefix(rawUrl, "#") {
					continue
				}

				parsedUrl, err := url.Parse(rawUrl)
				if err != nil {
					log.Printf("invalid URL %q: %v", rawUrl, err)
					continue
				}

				// Remove the url query params, removes duplicated urls
				// Example: localhost?lang=en and localhost?lang=fr are the same
				parsedUrl.RawQuery = ""

				var fullUrl string

				switch {
				case parsedUrl.IsAbs():
					if parsedUrl.Host != uri.Host {
						continue
					}

					fullUrl = parsedUrl.String()
				default:
					fullUrl = uri.ResolveReference(parsedUrl).String()
				}

				fullUrl = strings.TrimRight(fullUrl, "/")

				if _, exists := foundLinks[fullUrl]; !exists {
					foundLinks[fullUrl] = struct{}{}
				}
			}
		default:
			continue
		}
	}
}

func (c *Crawler) Fetch(rawURL string) (buffer *bytes.Buffer, link []string, err error) {
	uri, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse url: %w", err)
	}

	filename := alphanumericRegex.ReplaceAllString(rawURL, "_") + ".html"
	filename = filepath.Join(c.destinationDir, filename)

	contents, err := os.ReadFile(filename)

	switch {
	case err == nil:
		buffer = bytes.NewBuffer(contents)
	case os.IsNotExist(err):
		buffer, err = c.DownloadAndSave(uri.String(), filename)
		if err != nil {
			return nil, nil, fmt.Errorf("download and save: %w", err)
		}
	case !errors.Is(err, io.EOF):
		return nil, nil, fmt.Errorf("read file: %w", err)
	}

	links := c.FindLinks(uri, buffer)
	return buffer, links, nil
}

type fetchLinksResult struct {
	links []string
	err   error
}

func (c *Crawler) Crawl(rawURL string, depth int, wg *sync.WaitGroup) {
	c.mu.RLock()
	_, visited := c.visitedPages[rawURL]
	c.mu.RUnlock()

	if visited || depth <= 0 {
		return
	}

	c.mu.Lock()
	c.visitedPages[rawURL] = struct{}{}
	c.mu.Unlock()

	_, links, err := c.Fetch(rawURL)
	if err != nil {
		log.Printf("failed to fetch url: %s %v\n", rawURL, err)
		return
	}

	log.Printf("-- %s, found %d link(s)\n", rawURL, len(links))

	for _, link := range links {
		wg.Go(func() {
			c.Crawl(link, depth-1, wg)
		})
	}
}

func (c *Crawler) Start(rawURL string, depth int) []string {
	var wg sync.WaitGroup
	wg.Go(func() {
		c.Crawl(rawURL, depth, &wg)
	})

	wg.Wait()

	links := make([]string, 0, len(c.visitedPages))

	for link := range c.visitedPages {
		links = append(links, link)
	}

	return links

}

func NewCrawler(httpClient HttpClient, destinationDir string) (*Crawler, error) {
	if destinationDir == "" {
		destinationDir = DestinationDir
	}

	if err := os.MkdirAll(destinationDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	return &Crawler{
		destinationDir: destinationDir,
		httpClient:     httpClient,
		visitedPages:   make(map[string]struct{}),
	}, nil
}
