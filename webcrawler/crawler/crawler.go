package crawler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

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
// Used to sanitize URLs into safe filenames.
var alphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// DestinationDir is the default directory where fetched pages will be saved.
const DestinationDir = "storage"

// ErrPageNotFound is returned when an HTTP request returns a 404 status code.
var ErrPageNotFound = errors.New("page not found")

// HttpClient defines the interface for making HTTP requests.
type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Crawler is a concurrent web crawler that downloads HTML pages, extracts links,
// and follows them to a specified depth. It caches downloaded pages to disk
// to avoid redundant downloads and tracks visited URLs to prevent cycles.
//
// The Crawler is safe for concurrent use and provides mechanisms to limit
// the number of concurrent requests.
type Crawler struct {
	mu             sync.RWMutex
	httpClient     HttpClient
	destinationDir string
	visitedPages   map[string]struct{}
	maxConcurrent  int
}

// DownloadAndSave downloads the content from the given URI and saves it to the specified filename.
// It returns a buffer containing the downloaded content for immediate use.
func (c *Crawler) DownloadAndSave(ctx context.Context, uri string, filename string) (*bytes.Buffer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
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

// FindLinks extracts all valid links from an HTML document.
//
// It parses the HTML, finds all <a> tags with href attributes, and returns
// a list of absolute URLs that belong to the same host as the base URI.
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

// Fetch retrieves a page from the given URL, either from the disk cache or by downloading it.
//
// The function first checks if the page has been previously downloaded and cached.
// If the cached file exists, it reads from the disk. Otherwise, it downloads the page
// and saves it to the cache directory.
//
// After retrieving the content, it parses the HTML to extract all links.
func (c *Crawler) Fetch(ctx context.Context, rawURL string) (link []string, err error) {
	uri, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	filename := alphanumericRegex.ReplaceAllString(rawURL, "_")
	filename = filepath.Join(c.destinationDir, filename)

	contents, err := os.ReadFile(filename)

	buffer := &bytes.Buffer{}

	switch {
	case err == nil:
		buffer = bytes.NewBuffer(contents)
	case os.IsNotExist(err):
		buffer, err = c.DownloadAndSave(ctx, uri.String(), filename)
		if err != nil {
			return nil, fmt.Errorf("download and save: %w", err)
		}
	case !errors.Is(err, io.EOF):
		return nil, fmt.Errorf("read file: %w", err)
	}

	bufferCopy := bytes.NewBuffer(buffer.Bytes())

	links := c.FindLinks(uri, bufferCopy)
	return links, nil
}

// shouldVisit checks if a URL should be visited and marks it as visited atomically
func (c *Crawler) shouldVisit(rawURL string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, visited := c.visitedPages[rawURL]; visited {
		return false
	}

	c.visitedPages[rawURL] = struct{}{}
	return true
}

// Crawl recursively crawls web pages starting from the given URL to the specified depth.
//
// The function fetches the page at rawURL, extracts all links, and recursively
// crawls each link with depth-1. The crawling stops when the depth reaches 0 or when
// all reachable pages have been visited.
func (c *Crawler) Crawl(ctx context.Context, rawURL string, depth int, wg *sync.WaitGroup) {
	if depth <= 0 {
		return
	}

	if !c.shouldVisit(rawURL) {
		return
	}

	if ctx.Err() != nil {
		return
	}

	links, err := c.Fetch(ctx, rawURL)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("failed to fetch url: %s %v\n", rawURL, err)
		return
	}

	log.Printf("-- %s, found %d link(s)\n", rawURL, len(links))

	var semaphore = make(chan int, c.maxConcurrent)

	for _, link := range links {
		semaphore <- 1
		wg.Go(func() {
			c.Crawl(ctx, link, depth-1, wg)
			<-semaphore
		})
	}
}

func (c *Crawler) Start(ctx context.Context, rawURL string, depth int) []string {
	var wg sync.WaitGroup
	wg.Go(func() {
		c.Crawl(ctx, rawURL, depth, &wg)
	})

	wg.Wait()

	links := make([]string, 0, len(c.visitedPages))

	for link := range c.visitedPages {
		links = append(links, link)
	}

	return links

}

// NewCrawler creates a new Crawler instance with the specified configuration.
func NewCrawler(httpClient HttpClient, destinationDir string) (*Crawler, error) {
	if destinationDir == "" {
		destinationDir = DestinationDir
	}

	if err := os.MkdirAll(destinationDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &Crawler{
		destinationDir: destinationDir,
		httpClient:     httpClient,
		visitedPages:   make(map[string]struct{}),
		maxConcurrent:  runtime.NumCPU(),
	}, nil
}
