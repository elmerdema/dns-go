package main

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestRecordRRSupportsConfiguredTypes(t *testing.T) {
	tests := []struct {
		name     string
		record   Record
		expected uint16
	}{
		{"A", Record{Name: "a.example.", Type: dns.TypeA, Value: "192.0.2.1"}, dns.TypeA},
		{"AAAA", Record{Name: "aaaa.example.", Type: dns.TypeAAAA, Value: "2001:db8::1"}, dns.TypeAAAA},
		{"CNAME", Record{Name: "alias.example.", Type: dns.TypeCNAME, Value: "target.example."}, dns.TypeCNAME},
		{"TXT", Record{Name: "txt.example.", Type: dns.TypeTXT, Value: "hello"}, dns.TypeTXT},
		{"MX", Record{Name: "mx.example.", Type: dns.TypeMX, Value: "10 mail.example."}, dns.TypeMX},
		{"NS", Record{Name: "ns.example.", Type: dns.TypeNS, Value: "ns1.example."}, dns.TypeNS},
		{
			"SOA",
			Record{
				Name:  "example.",
				Type:  dns.TypeSOA,
				Value: "ns1.example. hostmaster.example. 1 3600 600 86400 300",
			},
			dns.TypeSOA,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, err := recordRR(test.record)
			if err != nil {
				t.Fatalf("recordRR() error = %v", err)
			}
			if record.Header().Rrtype != test.expected {
				t.Fatalf("record type = %d, want %d", record.Header().Rrtype, test.expected)
			}
		})
	}
}

func TestDNSHandlerResponseCodes(t *testing.T) {
	initSites()
	handler := DNSHandler{}

	tests := []struct {
		name   string
		qname  string
		qtype  uint16
		rcode  int
		answer int
	}{
		{"known A", "GO.DEV", dns.TypeA, dns.RcodeSuccess, 1},
		{"unknown A", "missing.example.", dns.TypeA, dns.RcodeNameError, 0},
		{"known name without requested type", "go.dev.", dns.TypeAAAA, dns.RcodeSuccess, 0},
		{"unsupported type", "go.dev.", dns.TypePTR, dns.RcodeNotImplemented, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := new(dns.Msg)
			request.SetQuestion(test.qname, test.qtype)
			writer := &captureResponseWriter{}

			handler.ServeDNS(writer, request)

			if writer.response == nil {
				t.Fatal("ServeDNS() did not write a response")
			}
			if writer.response.Rcode != test.rcode {
				t.Fatalf("response code = %d, want %d", writer.response.Rcode, test.rcode)
			}
			if len(writer.response.Answer) != test.answer {
				t.Fatalf("answer count = %d, want %d", len(writer.response.Answer), test.answer)
			}
		})
	}
}

type captureResponseWriter struct {
	response *dns.Msg
}

func (w *captureResponseWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{}
}

func (w *captureResponseWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{}
}

func (w *captureResponseWriter) WriteMsg(response *dns.Msg) error {
	w.response = response
	return nil
}

func (w *captureResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (w *captureResponseWriter) Close() error {
	return nil
}

func (w *captureResponseWriter) TsigStatus() error {
	return nil
}

func (w *captureResponseWriter) TsigTimersOnly(bool) {}

func (w *captureResponseWriter) Hijack() {}
