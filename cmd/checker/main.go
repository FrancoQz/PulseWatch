package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Result struct {
	URL     string
	Status  int
	Latency time.Duration
	Err     error
}

func check(url string) Result {
	start := time.Now()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return Result{URL: url, Err: err}
	}
	defer resp.Body.Close()

	return Result{
		URL:     url,
		Status:  resp.StatusCode,
		Latency: time.Since(start),
	}
}

func main() {
	urls := []string{
		"https://www.google.com",
		"https://www.uade.edu.ar",
		"https://github.com",
		"https://esto-no-existe-12345.com",
	}

	results := make(chan Result, len(urls))
	var wg sync.WaitGroup

	start := time.Now()

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			results <- check(u)
		}(url)
	}

	wg.Wait()
	close(results)

	for r := range results {
		if r.Err != nil {
			fmt.Printf("%-32s DOWN\n", r.URL)
			continue
		}
		fmt.Printf("%-32s %d  %v\n", r.URL, r.Status, r.Latency.Round(time.Millisecond))
	}

	fmt.Printf("\nTotal: %v\n", time.Since(start).Round(time.Millisecond))
}