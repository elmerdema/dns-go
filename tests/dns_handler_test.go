package tests

import (
	"context"
	"errors"
	"net"
	"testing"

	"dns-server/internal/dnsserver"

	"github.com/miekg/dns"
)

func TestDNSHandlerResponseCodes(t *testing.T) {
	store := newFakeRecordStore()
	handler := dnsserver.NewHandler(context.Background(), store)
	tests := []struct {
		name, qname   string
		qtype         uint16
		rcode, answer int
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

func TestDNSHandlerReadsRecordFromRedis(t *testing.T) {
	store := newFakeRecordStore()
	store.records[dnsserver.RecordKey("go.dev.", dns.TypeA)] = `{"Name":"go.dev.","Type":1,"TTL":120,"Value":"192.0.2.10"}`
	handler := dnsserver.NewHandler(context.Background(), store)

	request := new(dns.Msg)
	request.SetQuestion("go.dev.", dns.TypeA)
	writer := &captureResponseWriter{}
	handler.ServeDNS(writer, request)

	if writer.response.Rcode != dns.RcodeSuccess {
		t.Fatalf("response code = %d, want %d", writer.response.Rcode, dns.RcodeSuccess)
	}
	answer, ok := writer.response.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("answer type = %T, want *dns.A", writer.response.Answer[0])
	}
	if !answer.A.Equal(net.ParseIP("192.0.2.10")) {
		t.Fatalf("answer = %s, want 192.0.2.10", answer.A)
	}
}

func TestDNSHandlerReturnsServerFailureWhenRedisFails(t *testing.T) {
	store := newFakeRecordStore()
	store.err = errors.New("Redis unavailable")
	handler := dnsserver.NewHandler(context.Background(), store)

	request := new(dns.Msg)
	request.SetQuestion("go.dev.", dns.TypeA)
	writer := &captureResponseWriter{}
	handler.ServeDNS(writer, request)

	if writer.response.Rcode != dns.RcodeServerFailure {
		t.Fatalf("response code = %d, want %d", writer.response.Rcode, dns.RcodeServerFailure)
	}
}
