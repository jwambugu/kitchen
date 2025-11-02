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