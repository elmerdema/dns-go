package tests

import (
	"context"
	"encoding/json"
	"net"

	"dns-server/internal/dnsserver"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

type fakeRecordStore struct {
	records map[string]string
	err     error
}

func newFakeRecordStore() *fakeRecordStore {
	store := &fakeRecordStore{records: make(map[string]string)}
	for _, record := range dnsserver.DefaultRecords() {
		store.records[dnsserver.RecordKey(record.Name, record.Type)] = encodeRecordForTest(record)
		store.records[nameKeyForTest(record.Name)] = "1"
	}
	return store
}

func (s *fakeRecordStore) Get(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	value, ok := s.records[key]
	if !ok {
		return "", redis.Nil
	}
	return value, nil
}

func (s *fakeRecordStore) SetNX(_ context.Context, key, value string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if _, exists := s.records[key]; exists {
		return false, nil
	}
	s.records[key] = value
	return true, nil
}

func encodeRecordForTest(record dnsserver.Record) string {
	data, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func nameKeyForTest(name string) string {
	return "dns:record:" + dnsserver.NormalizeName(name) + ":_name"
}

type captureResponseWriter struct {
	response *dns.Msg
}

func (w *captureResponseWriter) LocalAddr() net.Addr  { return &net.UDPAddr{} }
func (w *captureResponseWriter) RemoteAddr() net.Addr { return &net.UDPAddr{} }
func (w *captureResponseWriter) WriteMsg(response *dns.Msg) error {
	w.response = response
	return nil
}
func (w *captureResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (w *captureResponseWriter) Close() error              { return nil }
func (w *captureResponseWriter) TsigStatus() error         { return nil }
func (w *captureResponseWriter) TsigTimersOnly(bool)       {}
func (w *captureResponseWriter) Hijack()                   {}
