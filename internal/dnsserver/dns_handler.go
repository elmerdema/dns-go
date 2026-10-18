package dnsserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	ctx              context.Context
	store            RecordStore
	recursionEnabled bool
	allowedClients   []netip.Prefix
	limiter          *clientLimiter
	maxResponseBytes int
}

func NewHandler(ctx context.Context, store RecordStore) Handler {
	return NewHandlerWithConfig(ctx, store, DefaultConfig())
}

func NewHandlerWithConfig(ctx context.Context, store RecordStore, config Config) Handler {
	if config.RateLimit <= 0 {
		config.RateLimit = DefaultConfig().RateLimit
	}
	if config.RateBurst <= 0 {
		config.RateBurst = DefaultConfig().RateBurst
	}
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = DefaultConfig().MaxResponseBytes
	}
	return Handler{
		ctx: ctx, store: store, recursionEnabled: config.RecursionEnabled,
		allowedClients: config.AllowedClients, limiter: newClientLimiter(config.RateLimit, config.RateBurst),
		maxResponseBytes: config.MaxResponseBytes,
	}
}

func (h Handler) ServeDNS(writer dns.ResponseWriter, request *dns.Msg) {
	if request == nil {
		return
	}
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = true
	response.RecursionAvailable = h.recursionEnabled

	client, ok := remoteIP(writer.RemoteAddr())
	if !ok || !allowedClient(client, h.allowedClients) {
		response.Rcode = dns.RcodeRefused
		_ = writer.WriteMsg(response)
		return
	}
	if !h.limiter.allow(client, time.Now()) {
		response.Rcode = dns.RcodeRefused
		_ = writer.WriteMsg(response)
		return
	}
	if len(request.Question) == 0 {
		response.Rcode = dns.RcodeFormatError
		_ = writer.WriteMsg(response)
		return
	}

	foundName := false
	unknownName := false
	unsupportedType := false

	for _, question := range request.Question {
		if strings.TrimSpace(question.Name) == "" {
			response.Rcode = dns.RcodeFormatError
			break
		}
		name := NormalizeName(question.Name)
		if _, ok := dns.IsDomainName(name); !ok {
			response.Rcode = dns.RcodeFormatError
			break
		}
		if !supportedType(question.Qtype) {
			unsupportedType = true
			continue
		}

		record, err := lookupRecord(h.ctx, h.store, name, question.Qtype)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				exists, existsErr := recordNameExists(h.ctx, h.store, name)
				if existsErr != nil {
					slog.Error("failed to check name in Redis", "name", name, "error", existsErr)
					response.Rcode = dns.RcodeServerFailure
					break
				}
				if !exists {
					unknownName = true
				}
				continue
			}
			slog.Error("failed to read record from Redis", "key", RecordKey(name, question.Qtype), "error", err)
			response.Rcode = dns.RcodeServerFailure
			break
		}

		rr, err := recordRR(record)
		if err != nil {
			slog.Error("invalid DNS record", "type", dns.TypeToString[record.Type], "name", name, "error", err)
			response.Rcode = dns.RcodeServerFailure
			break
		}
		foundName = true
		response.Answer = append(response.Answer, rr)
	}

	if response.Rcode == dns.RcodeSuccess {
		switch {
		case unsupportedType:
			response.Rcode = dns.RcodeNotImplemented
		case !foundName && unknownName:
			response.Rcode = dns.RcodeNameError
		}
	}

	response.Truncate(h.maxResponseBytes)
	if err := writer.WriteMsg(response); err != nil {
		slog.Error("failed to write DNS response", "error", err)
	}
}

func supportedType(qtype uint16) bool {
	switch qtype {
	case dns.TypeA, dns.TypeAAAA, dns.TypeCNAME, dns.TypeTXT, dns.TypeMX, dns.TypeNS, dns.TypeSOA:
		return true
	default:
		return false
	}
}

func recordRR(record Record) (dns.RR, error) {
	header := dns.RR_Header{Name: NormalizeName(record.Name), Rrtype: record.Type, Class: dns.ClassINET, Ttl: record.TTL}

	switch record.Type {
	case dns.TypeA:
		address := net.ParseIP(record.Value).To4()
		if address == nil {
			return nil, fmt.Errorf("invalid IPv4 address %q", record.Value)
		}
		return &dns.A{Hdr: header, A: address}, nil
	case dns.TypeAAAA:
		address := net.ParseIP(record.Value)
		if address == nil || address.To4() != nil {
			return nil, fmt.Errorf("invalid IPv6 address %q", record.Value)
		}
		return &dns.AAAA{Hdr: header, AAAA: address}, nil
	case dns.TypeCNAME:
		target, err := validatedName(record.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid CNAME target: %w", err)
		}
		return &dns.CNAME{Hdr: header, Target: target}, nil
	case dns.TypeTXT:
		return &dns.TXT{Hdr: header, Txt: []string{record.Value}}, nil
	case dns.TypeMX:
		fields := strings.Fields(record.Value)
		if len(fields) != 2 {
			return nil, fmt.Errorf("MX value must be '<priority> <target>'")
		}
		priority, err := strconv.ParseUint(fields[0], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid MX priority %q: %w", fields[0], err)
		}
		target, err := validatedName(fields[1])
		if err != nil {
			return nil, fmt.Errorf("invalid MX target: %w", err)
		}
		return &dns.MX{Hdr: header, Preference: uint16(priority), Mx: target}, nil
	case dns.TypeNS:
		target, err := validatedName(record.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid NS target: %w", err)
		}
		return &dns.NS{Hdr: header, Ns: target}, nil
	case dns.TypeSOA:
		fields := strings.Fields(record.Value)
		if len(fields) != 7 {
			return nil, fmt.Errorf("SOA value must be '<ns> <mailbox> <serial> <refresh> <retry> <expire> <minimum>'")
		}
		values := make([]uint32, 5)
		for index, field := range fields[2:] {
			value, err := strconv.ParseUint(field, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid SOA number %q: %w", field, err)
			}
			values[index] = uint32(value)
		}
		ns, err := validatedName(fields[0])
		if err != nil {
			return nil, fmt.Errorf("invalid SOA nameserver: %w", err)
		}
		mbox, err := validatedName(fields[1])
		if err != nil {
			return nil, fmt.Errorf("invalid SOA mailbox: %w", err)
		}
		return &dns.SOA{Hdr: header, Ns: ns, Mbox: mbox, Serial: values[0], Refresh: values[1], Retry: values[2], Expire: values[3], Minttl: values[4]}, nil
	default:
		return nil, fmt.Errorf("unsupported record type %d", record.Type)
	}
}

func validatedName(name string) (string, error) {
	normalized := NormalizeName(name)
	if _, ok := dns.IsDomainName(normalized); !ok {
		return "", fmt.Errorf("invalid domain name %q", name)
	}
	return normalized, nil
}
