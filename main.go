package main

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type DNSHandler struct {
}

func (d DNSHandler) ServeDNS(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = true

	for _, question := range request.Question {
		if question.Qtype != dns.TypeA {
			continue
		}

		name := strings.ToLower(question.Name)
		ip, found := sites[name]
		if found {
			fmt.Printf("The ip for %s has been found\n", ip)
			record := &dns.A{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: net.ParseIP(ip),
			}
			response.Answer = append(response.Answer, record)
			writer.WriteMsg(response)
		} else {
			fmt.Printf("No ip found for %s\n", name)
		}
	}
}

func main() {
	initSites()
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
