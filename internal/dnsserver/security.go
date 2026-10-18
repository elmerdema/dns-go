package dnsserver

import (
	"net"
	"net/netip"
	"sync"
	"time"
)

type clientLimiter struct {
	mu      sync.Mutex
	clients map[netip.Addr]*clientTokens
	rate    float64
	burst   float64
}

type clientTokens struct {
	tokens float64
	at     time.Time
}

func newClientLimiter(rate float64, burst int) *clientLimiter {
	return &clientLimiter{clients: make(map[netip.Addr]*clientTokens), rate: rate, burst: float64(burst)}
}

func (l *clientLimiter) allow(addr netip.Addr, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	state, ok := l.clients[addr]
	if !ok {
		state = &clientTokens{tokens: l.burst, at: now}
		l.clients[addr] = state
	}
	state.tokens += now.Sub(state.at).Seconds() * l.rate
	if state.tokens > l.burst {
		state.tokens = l.burst
	}
	state.at = now
	if state.tokens < 1 {
		return false
	}
	state.tokens--
	return true
}

func remoteIP(addr net.Addr) (netip.Addr, bool) {
	//parsing to extract the clients ip address from a DNS connection address
	// this lets the server identify querying clients
	// also rejects a list of clients outside DNS_ALLOWED_CLIENTS
	// rate limiting is applyed separately on each client IP using the function above
	if addr == nil {
		return netip.Addr{}, false
	}
	switch value := addr.(type) {
	case *net.UDPAddr:
		if value == nil {
			return netip.Addr{}, false
		}
		if len(value.IP) == 0 {
			return netip.MustParseAddr("127.0.0.1"), true
		}
		return netip.AddrFromSlice(value.IP)
	case *net.TCPAddr:
		if value == nil {
			return netip.Addr{}, false
		}
		if len(value.IP) == 0 {
			//empty ip address treats it as localhost
			return netip.MustParseAddr("127.0.0.1"), true
		}
		return netip.AddrFromSlice(value.IP)
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			parsed, parseErr := netip.ParseAddr(addr.String())
			return parsed, parseErr == nil
		}
		parsed, parseErr := netip.ParseAddr(host)
		return parsed, parseErr == nil
	}
}

func allowedClient(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
