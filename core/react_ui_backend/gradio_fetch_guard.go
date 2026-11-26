package react_ui_backend

// SSRF guard for getFile's URL fetches (gradio.go readFromURL).
//
// The request body names the URL, so without a guard any caller of the API
// can make this process fetch cloud metadata (169.254.169.254), other
// services on this machine (a database or Ollama on a loopback port) or
// other hosts of the LAN.
//
// The UI itself sends absolute URLs of its own origin (react-ui
// data/getWebuiURL.tsx: `${window.location.origin}/...`), which can be a
// loopback or a LAN address, so loopback and private addresses cannot be
// refused outright. The rule, checked when the connection is made (so a
// hostname that resolves to an internal address, a rebinding answer and a
// redirect are all covered):
//
//   - link-local (cloud metadata), unspecified and multicast addresses: never;
//   - loopback and private addresses: only on a port of this app (the port of
//     the request's Host, and the Gradio backend's port), and only when the
//     address is loopback or one of this machine's own addresses;
//   - every other address: allowed.

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// appPorts are the ports a fetch may reach on this machine: the one the UI
// was served from (the Host of the request) and the Gradio backend's.
func appPorts(selfHost, backend string) map[string]bool {
	ports := map[string]bool{}
	if selfHost != "" {
		if u, err := url.Parse("//" + selfHost); err == nil && u.Port() != "" {
			ports[u.Port()] = true
		} else {
			// A Host without a port: either default port can be meant.
			ports["80"], ports["443"] = true, true
		}
	}
	if u, err := url.Parse(backend); err == nil && u.Host != "" {
		switch {
		case u.Port() != "":
			ports[u.Port()] = true
		case u.Scheme == "https":
			ports["443"] = true
		default:
			ports["80"] = true
		}
	}
	return ports
}

// isLocalIP reports whether ip is an address of one of this machine's
// interfaces.
func isLocalIP(ip net.IP) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.Equal(ip) {
			return true
		}
	}
	return false
}

// guardDial decides whether a connection to address ("ip:port", already
// resolved) is allowed. isLocal says whether an IP is this machine's own.
func guardDial(address string, ports map[string]bool, isLocal func(net.IP) bool) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("bad address %q", address)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("bad address %q", address)
	}
	switch {
	case ip.IsUnspecified(), ip.IsMulticast(), ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		return fmt.Errorf("address %s is not allowed", host)
	case ip.IsLoopback(), ip.IsPrivate():
		if ports[port] && (ip.IsLoopback() || isLocal(ip)) {
			return nil
		}
		return fmt.Errorf("%s:%s is an internal address that is not part of this app", host, port)
	}
	return nil
}

// fileFetchClient is the client for getFile's URL fetches: gradioHTTP's
// settings (no timeout, no proxy, redirects followed) plus the dial guard.
// Keep-alive is off: the client lives for one call.
func fileFetchClient(selfHost, backend string) *http.Client {
	ports := appPorts(selfHost, backend)
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 30 * time.Second,
				Control: func(_, address string, _ syscall.RawConn) error {
					return guardDial(address, ports, isLocalIP)
				},
			}).DialContext,
			DisableKeepAlives: true,
		},
	}
}
