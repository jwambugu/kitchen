package crawler

import (
	"context"
	"fmt"
	"kitchen/pkg/assert"
	"kitchen/pkg/testutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

const testDestinationDir = "testdata"

func TestMain(m *testing.M) {
	code := m.Run()

	if err := os.RemoveAll(testDestinationDir); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "cleanup: %v\n", err)
	}

	os.Exit(code)
}

func TestCrawler_DownloadAndSave(t *testing.T) {
	var (
		ctx        = context.Background()
		httpClient = testutil.NewTestHttpClient()
	)

	crawler, err := NewCrawler(httpClient, testDestinationDir)
	assert.Nil(t, err)

	t.Run("downloads and saves the file", func(t *testing.T) {
		link := "http://localhost.com"

		httpClient.Request(link, func() (code int, body string) {
			return http.StatusOK, `
		<!DOCTYPE html>
			<html>
				<head>
					<title>Page Title</title>
				</head>
			<body>
				<h1>This is a Heading</h1>
				<p>This is a paragraph.</p>
			</body>
		</html>`
		})

		filename := filepath.Join(testDestinationDir, "localhost")

		buffer, err := crawler.DownloadAndSave(ctx, link, filename)
		assert.Nil(t, err)
		assert.NotNil(t, buffer)

		_, err = os.Stat(filename)
		assert.Nil(t, err)
	})

	t.Run("url does not exist", func(t *testing.T) {
		buffer, err := crawler.DownloadAndSave(ctx, "http://localghost.com", "localhost")
		assert.ErrorIs(t, err, ErrPageNotFound)
		assert.Nil(t, buffer)
	})

	t.Run("url does not exist", func(t *testing.T) {
		link := "http://localhost.com"

		httpClient.Request(link, func() (code int, body string) {
			return http.StatusInternalServerError, ""
		})

		buffer, err := crawler.DownloadAndSave(ctx, link, "localhost")
		assert.NotNil(t, err)
		assert.Nil(t, buffer)
	})
}

func TestCrawler_FindLinks(t *testing.T) {
	var (
		link       = "http://localhost.com"
		httpClient = testutil.NewTestHttpClient()
		ctx        = context.Background()
	)

	httpClient.Request(link, func() (code int, body string) {
		return http.StatusOK, `
			<ul>
				<a href="/">Home</a>
				<a href="/advanced-features">Advance features</a>
				<a href="/pricing">Pricing</a>
				<a href="/demo?url=staging">Demo</a>
				<a href="https://google.com"> External </a>
				<a href="mailto:someone@example.com">Send email</a>
				<a href="#">Go Home</a>
			</ul>`
	})

	crawler, err := NewCrawler(httpClient, testDestinationDir)
	assert.Nil(t, err)

	filename := filepath.Join(testDestinationDir, "localhost")

	buffer, err := crawler.DownloadAndSave(ctx, link, filename)
	assert.Nil(t, err)
	assert.NotNil(t, buffer)

	uri, err := url.Parse(link)
	assert.Nil(t, err)

	links := crawler.FindLinks(uri, buffer)
	assert.NotNil(t, links)
	assert.Equal[int](t, 3, len(links))
}

func TestCrawler_Crawl(t *testing.T) {
	var (
		link       = "http://localhost.com"
		httpClient = testutil.NewTestHttpClient()
		ctx        = context.Background()
	)

	httpClient.Request(link, func() (code int, body string) {
		return http.StatusOK, `
			<ul>
				<a href="/">Home</a>
				<a href="/advanced-features">Advance features</a>
				<a href="/pricing">Pricing</a>
				<a href="/demo?url=staging">Demo</a>
				<a href="https://google.com"> External </a>
				<a href="mailto:someone@example.com">Send email</a>
				<a href="#">Go Home</a>
			</ul>`
	})

	crawler, err := NewCrawler(httpClient, testDestinationDir)
	assert.Nil(t, err)

	links := crawler.Start(ctx, link, 10)
	assert.Equal(t, len(links), 4)
}
