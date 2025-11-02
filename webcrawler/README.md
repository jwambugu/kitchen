## Implement a recursive,mirroring web crawler

The crawler should be a command-line tool that accepts a starting URL and a
destination directory. The crawler will then download the page at the URL, save it in
the destination directory, and then recursively proceed to any valid links in this page.

A valid link is the value of a href attribute in an `<a>` tag the resolves to urls that are children of the initial URL.
For example, given initial URL https://start.url/abc, URL that resolves to https://start.url/abc/foo and
https://start.url/abc/foo/bar are valid URLs, but ones that resolve to https://another.domain or https://start.url 
are not valid URLs and should be skipped.

Additionally, the crawler should:
- Correctly handle being interrupted by Ctrl-C
- Perform work in parallel where reasonable
- Support resume functionality by checking the destination directory for downloaded pages and skip downloading and
processing where not necessary
- Provide “happy-path” test coverage


# Web Crawler

A recursive, concurrent web crawler that mirrors websites by downloading pages and following links within a specified URL path.

## Features

✅ **Path-based filtering** - Only crawls URLs that are children of the starting URL  
✅ **Concurrent crawling** - Parallel downloads with configurable concurrency limits  
✅ **Resume support** - Checks cache directory and skips already downloaded pages  
✅ **Graceful shutdown** - Handles Ctrl-C (SIGINT) to stop cleanly  
✅ **Comprehensive tests** - Happy path test coverage with mock HTTP client

## Requirements

- Go 1.23 or later (uses `sync.WaitGroup.Go()`)
- `golang.org/x/net/html` package

## Installation

```bash
# Install dependencies
go mod tidy

# Build the CLI tool
go build -o crawler cmd/crawler/main.go
```

## Usage

### Basic Command

```bash
./crawler -url https://example.com/docs -dir ./mirror -depth 3
```

### Command-line Flags

- `-url` (required) - Starting URL to crawl
- `-dir` (default: "storage") - Destination directory for downloaded pages
- `-depth` (default: 3) - Maximum crawl depth

### Examples

**Crawl a documentation site:**
```bash
./crawler -url https://example.com/docs -dir ./docs-mirror -depth 5
```

**Limited concurrency for slower networks:**
```bash
./crawler -url https://example.com/blog -dir ./blog-mirror
```

**Resume interrupted crawl:**
```bash
# The same command will skip already downloaded pages
./crawler -url https://example.com/docs -dir ./docs-mirror -depth 5
```

**Stop with Ctrl-C:**
Press `Ctrl-C` to gracefully stop the crawl. Already downloaded pages are saved and the crawl can be resumed by running the same command again.

## URL Filtering Logic

The crawler only follows links that are **children** of the starting URL:

Given starting URL: `https://example.com/docs`

✅ **Crawled:**
- `https://example.com/docs/guide`
- `https://example.com/docs/api/reference`

❌ **Skipped:**
- `https://example.com/blog` (different path)
- `https://example.com` (parent path)
- `https://other-domain.com` (different domain)

## Running Tests

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -cover ./...
```

## How It Works

1. **Parse starting URL** and validate it's a valid HTTP/HTTPS URL
2. **Check cache** - If page already exists in destination directory, read from disk
3. **Download page** - If not cached, fetch via HTTP and save to disk
4. **Extract links** - Parse HTML and find all `<a>` tags with `href` attributes
5. **Filter links** - Keep only URLs that are children of the starting URL
6. **Recurse** - Spawn goroutines to crawl each valid link with depth-1
7. **Coordinate** - Use WaitGroup to wait for all goroutines to complete

### Concurrency Control

- Uses a semaphore pattern to limit concurrent HTTP requests
- Semaphore is acquired before fetching, released immediately after
- Child goroutines can spawn without blocking on semaphore
- Prevents resource exhaustion on large sites

### Resume Functionality

- Each URL is converted to a safe filename (non-alphanumeric → `_`)
- Before downloading, checks if file exists in destination directory
- If a file exists, reads from the disk instead of making HTTP request
- Same visited-pages tracking prevents re-processing


### URL Normalization
- Query parameters removed (e.g., `?lang=en`)
- Trailing slashes removed
- Relative URLs resolved to absolute
- Prevents duplicate crawling of the same logical page

### Error Handling
- 404 errors return `ErrPageNotFound`
- Other HTTP errors logged but don't stop crawl
- Context cancellation handled gracefully
- File I/O errors properly propagated
