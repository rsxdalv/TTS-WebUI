package react_ui_backend

// LAN mode (TTS_GO_PROXY_LISTEN=lan, D18): the proxy listens on every
// interface so other devices can open the UI. The Host and Origin guards then
// also accept this machine's own interface addresses and the allowlisted host
// names; every other host is still refused, which keeps DNS rebinding out.

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// lanHosts is the set of extra host names LAN mode accepts.
type lanHosts struct {
	allowed map[string]bool // from TTS_GO_PROXY_ALLOWED_HOSTS

	mu        sync.Mutex
	addrs     map[string]bool // this machine's interface addresses
	refreshed time.Time
	lookup    func() ([]net.IP, error) // replaced in tests
}

// refreshEvery limits how often a miss rereads the interface addresses, which
// can change when DHCP renews or a network comes up.
const refreshEvery = 5 * time.Second

func newLANHosts(allowed []string) *lanHosts {
	l := &lanHosts{allowed: map[string]bool{}, lookup: interfaceIPs}
	for _, h := range allowed {
		l.allowed[normalizeHost(h)] = true
	}
	return l
}

// accepts reports whether name (no port) is one of this machine's addresses or
// an allowlisted host.
func (l *lanHosts) accepts(name string) bool {
	name = normalizeHost(name)
	if name == "" {
		return false
	}
	if l.allowed[name] {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.addrs[name] {
		return true
	}
	if time.Since(l.refreshed) < refreshEvery && l.addrs != nil {
		return false
	}
	l.refreshed = time.Now()
	ips, err := l.lookup()
	if err != nil {
		return false
	}
	l.addrs = map[string]bool{}
	for _, ip := range ips {
		l.addrs[ip.String()] = true
	}
	return l.addrs[name]
}

// normalizeHost lower-cases a host name and writes IP addresses in their
// canonical form. Zoned IPv6 addresses are refused.
func normalizeHost(h string) string {
	h = strings.ToLower(strings.Trim(h, "[]"))
	if strings.Contains(h, "%") {
		return ""
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.String()
	}
	return h
}

// interfaceIPs returns the addresses of this machine's interfaces.
func interfaceIPs() ([]net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			ips = append(ips, n.IP)
		}
	}
	return ips, nil
}

// lanOriginOK accepts http://<accepted host>:<port> with nothing after it.
func (h *handler) lanOriginOK(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	if u.Port() != h.port || u.Host != net.JoinHostPort(u.Hostname(), u.Port()) {
		return false
	}
	return h.lan.accepts(u.Hostname())
}

// LANURLs returns the addresses another device can use to open the UI: one
// http URL per non-loopback IPv4 address of this machine, private ranges first.
func LANURLs(port int) []string {
	ips, err := interfaceIPs()
	if err != nil {
		return nil
	}
	var private, public []string
	for _, ip := range ips {
		ip4 := ip.To4()
		if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() || ip4.IsUnspecified() {
			continue
		}
		u := "http://" + net.JoinHostPort(ip4.String(), strconv.Itoa(port))
		if ip4.IsPrivate() {
			private = append(private, u)
		} else {
			public = append(public, u)
		}
	}
	sort.Strings(private)
	sort.Strings(public)
	return append(private, public...)
}

// LANWarning is the one line the launcher logs when LAN mode is on.
func LANWarning(port int) string {
	urls := LANURLs(port)
	where := "this machine's network address on port " + strconv.Itoa(port)
	if len(urls) > 0 {
		where = strings.Join(urls, " or ")
	}
	return fmt.Sprintf("WARNING: the React UI is reachable from the network at %s, with no password. Anyone on your network can use it. Unset TTS_GO_PROXY_LISTEN to keep it on this computer only.", where)
}

// listenAll binds every interface, IPv4 and, where available, IPv6.
func listenAll(port int) ([]net.Listener, error) {
	v4, err := net.Listen("tcp4", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("listening on port %d: %w", port, err)
	}
	listeners := []net.Listener{v4}

	probe, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return listeners, nil // no IPv6 on this machine
	}
	_ = probe.Close()

	v6Port := v4.Addr().(*net.TCPAddr).Port
	v6, err := net.Listen("tcp6", net.JoinHostPort("::", strconv.Itoa(v6Port)))
	if err != nil {
		_ = v4.Close()
		return nil, fmt.Errorf("listening on [::]:%d: %w", v6Port, err)
	}
	return append(listeners, v6), nil
}
