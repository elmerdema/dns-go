package main

var sites map[string]string

func initSites() {
	sites = map[string]string{
		"https://go.dev/blog/maps": "127.0.0.1",
		"https://go.dev/blog":      "127.0.0.2",
		"https://go.dev":           "127.0.0.3",
	}
}
