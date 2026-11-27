package mcp

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// MinTokenLen is the shortest bearer token HTTP mode accepts.
const MinTokenLen = 16

// CheckListen refuses an HTTP address that is not loopback, or a missing or
// short token. No token means no listener.
func CheckListen(addr, token string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("--http %q: %w", addr, err)
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("--http must be a loopback address such as 127.0.0.1:7779")
		}
	}
	if len(token) < MinTokenLen {
		return fmt.Errorf("HTTP mode needs a token of at least %d characters (see --token-env)", MinTokenLen)
	}
	return nil
}

// Handler serves the MCP server over Streamable HTTP. A request needs the
// bearer token; the SDK rejects a non-loopback Host (DNS rebinding) and the
// standard library rejects a cross-origin browser request.
func Handler(s *sdk.Server, token string) http.Handler {
	inner := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s }, &sdk.StreamableHTTPOptions{
		CrossOriginProtection: http.NewCrossOriginProtection(),
		MaxRequestBodyBytes:   1 << 20,
	})
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(w, r)
	})
}
