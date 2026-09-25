package audit

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type peerAddrKey struct{}

// maxAddrLength bounds what is stored from client-controlled headers.
const maxAddrLength = 64

// clientIPHeaders are read in order.
var clientIPHeaders = []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"}

// CapturePeerAddr keeps the TCP peer address in the context. It must be registered before
// chi's RealIP, which overwrites r.RemoteAddr.
func CapturePeerAddr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), peerAddrKey{}, hostOf(r.RemoteAddr))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// PeerAddr returns the address captured by CapturePeerAddr, or the one of the request.
func PeerAddr(r *http.Request) string {
	if peer, ok := r.Context().Value(peerAddrKey{}).(string); ok {
		return peer
	}
	return hostOf(r.RemoteAddr)
}

// ClientIP returns the address of the client behind the proxies, falling back to the peer.
func ClientIP(r *http.Request) string {
	for _, header := range clientIPHeaders {
		// X-Forwarded-For lists every hop, the client first.
		first, _, _ := strings.Cut(r.Header.Get(header), ",")
		if ip := strings.TrimSpace(first); ip != "" {
			return truncate(ip)
		}
	}
	return PeerAddr(r)
}

func hostOf(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return truncate(addr)
	}
	return truncate(host)
}

func truncate(s string) string {
	if len(s) > maxAddrLength {
		return s[:maxAddrLength]
	}
	return s
}
