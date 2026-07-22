package main

import (
	"fmt"
	"net/http"
	"time"
)

func check(url string) {
	start := time.Now()

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("%-30s DOWN  (%v)\n", url, err)
		return
	}
	defer resp.Body.Close()

	latency := time.Since(start)
	fmt.Printf("%-30s %d  %v\n", url, resp.StatusCode, latency.Round(time.Millisecond))
}

func main() {
	urls := []string{
		"https://www.google.com",
		"https://www.uade.edu.ar",
		"https://esto-no-existe-12345.com",
	}

	for _, url := range urls {
		check(url)
	}
}