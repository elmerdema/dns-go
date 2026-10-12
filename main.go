package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

type DNSHandler struct {
}

func (d DNSHandler) ServeDNS(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = true

	foundName := false
	unknownName := false
	unsupportedType := false

	for _, question := range request.Question {
		name := normalizeName(question.Name)
		if !supportedType(question.Qtype) {
			unsupportedType = true
			continue
		}

		nameRecords, found := records[name]
		if !found {
			unknownName = true
			continue
		}

		for _, record := range nameRecords {
			if record.Type != question.Qtype {
				continue
			}

			rr, err := recordRR(record)
			if err != nil {
				log.Printf("invalid %s record for %s: %v", dns.TypeToString[record.Type], name, err)
				response.Rcode = dns.RcodeServerFailure
				break
			}

			foundName = true
			response.Answer = append(response.Answer, rr)
		}
		if response.Rcode == dns.RcodeServerFailure {
			break
		}
	}

	if response.Rcode == dns.RcodeSuccess {
		switch {
		case unsupportedType:
			response.Rcode = dns.RcodeNotImplemented
		case !foundName && unknownName:
			response.Rcode = dns.RcodeNameError
		}
	}

	if err := writer.WriteMsg(response); err != nil {
		log.Printf("failed to write DNS response: %v", err)
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
	header := dns.RR_Header{
		Name:   normalizeName(record.Name),
		Rrtype: record.Type,
		Class:  dns.ClassINET,
		Ttl:    record.TTL,
	}

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
		return &dns.CNAME{Hdr: header, Target: normalizeName(record.Value)}, nil
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
		return &dns.MX{Hdr: header, Preference: uint16(priority), Mx: normalizeName(fields[1])}, nil
	case dns.TypeNS:
		return &dns.NS{Hdr: header, Ns: normalizeName(record.Value)}, nil
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
		return &dns.SOA{
			Hdr:     header,
			Ns:      normalizeName(fields[0]),
			Mbox:    normalizeName(fields[1]),
			Serial:  values[0],
			Refresh: values[1],
			Retry:   values[2],
			Expire:  values[3],
			Minttl:  values[4],
		}, nil
	default:
		return nil, fmt.Errorf("unsupported record type %d", record.Type)
	}
}

func main() {
	initSites()
	client, ctx := initRedisClient()

	for key, nameRecords := range records {
		for _, record := range nameRecords {
			value := fmt.Sprintf("%s %s", dns.TypeToString[record.Type], record.Value)
			setElement(client, ctx, key+dns.TypeToString[record.Type], value)
			fmt.Printf("%s -> %s\n", key, storeElement(ctx, client, key+dns.TypeToString[record.Type]))
		}
	}

	mux := dns.NewServeMux()
	mux.Handle(".", DNSHandler{})

	server := &dns.Server{
		Addr:    "127.0.0.1:8503",
		Net:     "udp",
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
