package main

import (
	"log"

	"dns-server/internal/dnsserver"
	"github.com/miekg/dns"
)

func main() {
	records := dnsserver.DefaultRecords()
	client, ctx := dnsserver.InitRedisClient()
	defer client.Close()

	store := dnsserver.NewRedisStore(client)
	if err := dnsserver.SeedRecords(ctx, records, store); err != nil {
		log.Printf("failed to seed Redis records: %v", err)
	}

	mux := dns.NewServeMux()
	mux.Handle(".", dnsserver.NewHandler(ctx, store))

	server := &dns.Server{
		Addr:    "127.0.0.1:8503",
		Net:     "udp",
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
