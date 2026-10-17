package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"dns-server/internal/dnsserver"

	"github.com/miekg/dns"
)

func main() {
	config, err := dnsserver.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	level := new(slog.LevelVar)
	switch config.LogLevel {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	records, err := dnsserver.LoadRecords(config.ZoneFile, config.DefaultTTL)
	if err != nil {
		log.Fatalf("failed to load records: %v", err)
	}

	client, redisContext := dnsserver.InitRedisClientWithConfig(config)
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("failed to close Redis client: %v", err)
		}
	}()

	store := dnsserver.NewRedisStore(client)
	if err := dnsserver.SeedRecords(redisContext, records, store); err != nil {
		log.Fatalf("failed to seed Redis records: %v", err)
	}

	mux := dns.NewServeMux()
	mux.Handle(".", dnsserver.NewHandlerWithConfig(context.Background(), store, config))
	servers := []*dns.Server{
		{Addr: config.ListenAddr, Net: "udp", Handler: mux},
		{Addr: config.ListenAddr, Net: "tcp", Handler: mux},
	}

	runErr := make(chan error, len(servers))
	for _, server := range servers {
		go func(server *dns.Server) {
			if err := server.ListenAndServe(); err != nil {
				runErr <- err
			}
		}(server)
	}

	shutdown := func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
		defer cancel()
		for _, server := range servers {
			if err := server.ShutdownContext(shutdownContext); err != nil {
				log.Printf("failed to shut down %s DNS server: %v", server.Net, err)
			}
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case err := <-runErr:
		if err != nil {
			log.Printf("DNS server failed: %v", err)
		}
		shutdown()
	case signal := <-signals:
		log.Printf("received %s, shutting down", signal)
		shutdown()
	}
}

//ss
