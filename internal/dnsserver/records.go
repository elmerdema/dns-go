package dnsserver

import (
	"strings"

	"github.com/miekg/dns"
)

type Record struct {
	Name  string
	Type  uint16
	TTL   uint32
	Value string
}

func DefaultRecords() []Record {
	return []Record{
		{Name: "go.dev.", Type: dns.TypeA, TTL: 60, Value: "127.0.0.1"},
		{Name: "google.com.", Type: dns.TypeA, TTL: 60, Value: "127.0.0.2"},
		{Name: "apple.com.", Type: dns.TypeA, TTL: 60, Value: "127.0.0.3"},
		{Name: "example.com.", Type: dns.TypeAAAA, TTL: 300, Value: "2001:db8::1"},
		{Name: "www.example.com.", Type: dns.TypeCNAME, TTL: 300, Value: "example.com."},
		{Name: "example.com.", Type: dns.TypeTXT, TTL: 300, Value: "managed by dns-go"},
		{Name: "example.com.", Type: dns.TypeMX, TTL: 300, Value: "10 mail.example.com."},
		{Name: "example.com.", Type: dns.TypeNS, TTL: 300, Value: "ns1.example.com."},
		{Name: "example.com.", Type: dns.TypeSOA, TTL: 300, Value: "ns1.example.com. hostmaster.example.com. 1 3600 600 86400 300"},
	}
}

func NormalizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	return name
}
