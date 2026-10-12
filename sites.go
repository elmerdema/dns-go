package main

import (
	"context"
	"strings"

	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

// terst redis connection
//docker run --name redis-dev \  -p 6379:6379 \  -d redis:7
// docker exec -it redis-dev redis-cli ping

type Record struct {
	Name  string
	Type  uint16
	TTL   uint32
	Value string
}

var records map[string][]Record

func initSites() {
	records = make(map[string][]Record)
	addRecord := func(record Record) {
		record.Name = normalizeName(record.Name)
		if record.TTL == 0 {
			record.TTL = 60
		}
		records[record.Name] = append(records[record.Name], record)
	}

	addRecord(Record{Name: "go.dev.", Type: dns.TypeA, TTL: 60, Value: "127.0.0.1"})
	addRecord(Record{Name: "google.com.", Type: dns.TypeA, TTL: 60, Value: "127.0.0.2"})
	addRecord(Record{Name: "apple.com.", Type: dns.TypeA, TTL: 60, Value: "127.0.0.3"})
	addRecord(Record{Name: "example.com.", Type: dns.TypeAAAA, TTL: 300, Value: "2001:db8::1"})
	addRecord(Record{Name: "www.example.com.", Type: dns.TypeCNAME, TTL: 300, Value: "example.com."})
	addRecord(Record{Name: "example.com.", Type: dns.TypeTXT, TTL: 300, Value: "managed by dns-go"})
	addRecord(Record{Name: "example.com.", Type: dns.TypeMX, TTL: 300, Value: "10 mail.example.com."})
	addRecord(Record{Name: "example.com.", Type: dns.TypeNS, TTL: 300, Value: "ns1.example.com."})
	addRecord(Record{
		Name:  "example.com.",
		Type:  dns.TypeSOA,
		TTL:   300,
		Value: "ns1.example.com. hostmaster.example.com. 1 3600 600 86400 300",
	})
}

func normalizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	return name
}

func initRedisClient() (*redis.Client, context.Context) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0, // use default DB
		Protocol: 2,
	})
	ctx := context.Background()
	return rdb, ctx
}

func storeElement(ctx context.Context, rdb *redis.Client, key string) string {
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		panic(err)
	}
	return val
}

func setElement(rdb *redis.Client, ctx context.Context, key string, value string) {
	err := rdb.Set(ctx, key, value, 0).Err()
	if err != nil {
		panic(err)
	}
}
