package main

var sites map[string]string

func initSites() {
	sites = map[string]string{
		"go.dev.":     "127.0.0.1",
		"google.com.": "127.0.0.2",
		"apple.com.":  "127.0.0.3",
	}
}
