package switches

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestDefaultsAfterCutover(t *testing.T) {
	for _, e := range []map[string]string{nil, {EnvProxy: "", EnvRoutes: ""}, {EnvProxy: "1"}} {
		s, err := Parse(env(e))
		if err != nil {
			t.Fatal(err)
		}
		if !s.Proxy || !s.Route("upload") || !s.Route("data-outputs") || !s.Route("anything") {
			t.Fatalf("%v: want the proxy and every claim on, got %+v", e, s)
		}
		if s.ProxyLAN {
			t.Fatalf("%v: LAN mode must stay opt-in", e)
		}
	}
}

func TestOptOuts(t *testing.T) {
	s, err := Parse(env(map[string]string{EnvProxy: "0"}))
	if err != nil || s.Proxy || s.Route("upload") {
		t.Fatalf("proxy off: %+v %v", s, err)
	}
	s, err = Parse(env(map[string]string{EnvRoutes: "none"}))
	if err != nil || !s.Proxy || s.Route("upload") || s.Route("gradio") {
		t.Fatalf("routes none: %+v %v", s, err)
	}
	s, err = Parse(env(map[string]string{EnvProxy: "0", EnvRoutes: "none"}))
	if err != nil || s.Proxy || s.Route("upload") {
		t.Fatalf("proxy off and routes none: %+v %v", s, err)
	}
	s, err = Parse(env(map[string]string{EnvRoutes: "upload"}))
	if err != nil || !s.Route("upload") || s.Route("gradio") {
		t.Fatalf("a route list limits the claims: %+v %v", s, err)
	}
}

func TestEnabledNames(t *testing.T) {
	s, err := Parse(env(map[string]string{
		EnvProxy:  "1",
		EnvRoutes: "upload, gradio",
		EnvSteps:  "mamba",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !s.Proxy || !s.Route("upload") || !s.Route("gradio") || s.Route("data-outputs") {
		t.Fatalf("routes wrong: %+v", s)
	}
	if !s.Step("mamba") || s.Step("install") {
		t.Fatalf("steps wrong: %+v", s)
	}
}

func TestAll(t *testing.T) {
	s, err := Parse(env(map[string]string{EnvProxy: "1", EnvRoutes: "all", EnvSteps: "all"}))
	if err != nil {
		t.Fatal(err)
	}
	if !s.Route("anything") || !s.Step("anything") {
		t.Fatalf("all did not enable: %+v", s)
	}
}

func TestRejected(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"proxy typo", map[string]string{EnvProxy: "true"}, EnvProxy},
		{"routes with proxy off", map[string]string{EnvProxy: "0", EnvRoutes: "upload"}, "needs the proxy"},
		{"all routes with proxy off", map[string]string{EnvProxy: "0", EnvRoutes: "all"}, "needs the proxy"},
		{"none mixed with names", map[string]string{EnvRoutes: "none,upload"}, "invalid name"},
		{"bad route name", map[string]string{EnvProxy: "1", EnvRoutes: "Up_load"}, "invalid name"},
		{"empty list item", map[string]string{EnvSteps: "mamba,,install"}, "invalid name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(env(c.env))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestCheckKnownNames(t *testing.T) {
	s, err := Parse(env(map[string]string{EnvProxy: "1", EnvRoutes: "upload", EnvSteps: "mamba"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CheckRoutes([]string{"upload"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckRoutes([]string{"gradio"}); err == nil {
		t.Fatal("unknown route accepted")
	}
	if err := s.CheckSteps([]string{"install"}); err == nil {
		t.Fatal("unknown step accepted")
	}
}

func TestProxyListen(t *testing.T) {
	s, err := Parse(env(nil))
	if err != nil || s.ProxyLAN || s.ProxyAllowedHosts != nil {
		t.Fatalf("default: %+v %v", s, err)
	}
	for _, v := range []string{"", "loopback", "LOOPBACK"} {
		s, err := Parse(env(map[string]string{EnvProxy: "1", EnvProxyListen: v}))
		if err != nil || s.ProxyLAN {
			t.Fatalf("%q: %+v %v", v, s, err)
		}
	}
	s, err = Parse(env(map[string]string{EnvProxy: "1", EnvProxyListen: "lan", EnvProxyAllowedHosts: " MyPC.local, 192.168.1.20 ,::1,[fe80::1]"}))
	if err != nil || !s.ProxyLAN {
		t.Fatalf("lan: %+v %v", s, err)
	}
	if got := strings.Join(s.ProxyAllowedHosts, " "); got != "mypc.local 192.168.1.20 ::1 fe80::1" {
		t.Fatalf("hosts: %q", got)
	}
}

func TestProxyListenRejected(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"unknown listen value", map[string]string{EnvProxy: "1", EnvProxyListen: "all"}, "loopback or lan"},
		{"lan with proxy off", map[string]string{EnvProxy: "0", EnvProxyListen: "lan"}, "needs the proxy"},
		{"hosts without lan", map[string]string{EnvProxy: "1", EnvProxyAllowedHosts: "mypc"}, "needs TTS_GO_PROXY_LISTEN=lan"},
		{"host with port", map[string]string{EnvProxy: "1", EnvProxyListen: "lan", EnvProxyAllowedHosts: "mypc:3000"}, "invalid host"},
		{"wildcard", map[string]string{EnvProxy: "1", EnvProxyListen: "lan", EnvProxyAllowedHosts: "*"}, "invalid host"},
		{"empty item", map[string]string{EnvProxy: "1", EnvProxyListen: "lan", EnvProxyAllowedHosts: "a,,b"}, "invalid host"},
		{"url", map[string]string{EnvProxy: "1", EnvProxyListen: "lan", EnvProxyAllowedHosts: "http://mypc"}, "invalid host"},
		{"ipv6 zone", map[string]string{EnvProxy: "1", EnvProxyListen: "lan", EnvProxyAllowedHosts: "fe80::1%eth0"}, "invalid host"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(env(c.env))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}
