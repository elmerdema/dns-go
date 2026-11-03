package admin

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"dns-server/internal/dnsserver"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	store dnsserver.RecordAdminStore
	token string
}

type recordRequest struct {
	TTL   uint32 `json:"ttl"`
	Value string `json:"value"`
}

type recordResponse struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	TTL   uint32 `json:"ttl"`
	Value string `json:"value"`
}

func NewServer(store dnsserver.RecordAdminStore, token string) *Server {
	return &Server{store: store, token: token}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/readyz", s.ready)
	mux.HandleFunc("/api/v1/records", s.records)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) records(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listRecords(w, r)
	case http.MethodPut:
		s.upsertRecord(w, r)
	case http.MethodDelete:
		s.deleteRecord(w, r)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) listRecords(w http.ResponseWriter, r *http.Request) {
	records, err := s.store.ListRecords(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list records")
		return
	}
	filterName := r.URL.Query().Get("name")
	filterType := strings.ToUpper(r.URL.Query().Get("type"))
	var filterRecordType uint16
	if filterType != "" {
		var ok bool
		filterRecordType, ok = dns.StringToType[filterType]
		if !ok || !isSupportedType(filterRecordType) {
			writeError(w, http.StatusBadRequest, "type must be a supported DNS record type")
			return
		}
	}
	result := make([]recordResponse, 0, len(records))
	for _, record := range records {
		if filterName != "" && dnsserver.NormalizeName(filterName) != dnsserver.NormalizeName(record.Name) {
			continue
		}
		if filterType != "" && record.Type != filterRecordType {
			continue
		}
		result = append(result, toResponse(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": result})
}

func (s *Server) upsertRecord(w http.ResponseWriter, r *http.Request) {
	name, recordType, ok := queryRecordIdentity(w, r)
	if !ok {
		return
	}
	var request recordRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}
	record := dnsserver.Record{Name: name, Type: recordType, TTL: request.TTL, Value: request.Value}
	if err := dnsserver.ValidateRecord(record); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.UpsertRecord(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store record")
		return
	}
	writeJSON(w, http.StatusOK, toResponse(record))
}

func (s *Server) deleteRecord(w http.ResponseWriter, r *http.Request) {
	name, recordType, ok := queryRecordIdentity(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteRecord(r.Context(), name, recordType); err != nil {
		if errors.Is(err, redis.Nil) {
			writeError(w, http.StatusNotFound, "record not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete record")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func queryRecordIdentity(w http.ResponseWriter, r *http.Request) (string, uint16, bool) {
	name := r.URL.Query().Get("name")
	typeName := strings.ToUpper(r.URL.Query().Get("type"))
	recordType, ok := dns.StringToType[typeName]
	if name == "" || !ok || !isSupportedType(recordType) {
		writeError(w, http.StatusBadRequest, "name and supported type query parameters are required")
		return "", 0, false
	}
	return dnsserver.NormalizeName(name), recordType, true
}

func isSupportedType(recordType uint16) bool {
	switch recordType {
	case dns.TypeA, dns.TypeAAAA, dns.TypeCNAME, dns.TypeTXT, dns.TypeMX, dns.TypeNS, dns.TypeSOA:
		return true
	default:
		return false
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "Redis is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) || s.token == "" {
		return false
	}
	provided := []byte(strings.TrimSpace(strings.TrimPrefix(header, prefix)))
	expected := []byte(s.token)
	return len(provided) == len(expected) &&
		subtle.ConstantTimeCompare(provided, expected) == 1
}

func toResponse(record dnsserver.Record) recordResponse {
	return recordResponse{Name: dnsserver.NormalizeName(record.Name), Type: dns.TypeToString[record.Type], TTL: record.TTL, Value: record.Value}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": strconv.Itoa(status)})
}
