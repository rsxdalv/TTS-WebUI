package launcher

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/router"
)

// writeCert writes a self-signed certificate for 127.0.0.1 and its key (PEM,
// encrypted with password when it is not empty) and returns their paths.
func writeCert(t *testing.T, password string) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	block := &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}
	if password != "" {
		//lint:ignore SA1019 builds the legacy encrypted PEM a password-protected ssl_keyfile has
		if block, err = x509.EncryptPEMBlock(rand.Reader, block.Type, keyDER, []byte(password), x509.PEMCipherAES256); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

// serveRouterOver starts a router in front of a plain backend on ln.
func serveRouterOver(t *testing.T, ln net.Listener) {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "gradio says "+r.Header.Get("X-Forwarded-Proto"))
	}))
	t.Cleanup(backend.Close)
	rt := router.New(router.Options{Token: "t"})
	if err := rt.AddRoute("/", backend.URL); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = rt.ServeListener(ctx, ln) }()
	t.Cleanup(func() { cancel(); <-done })
}

// With ssl_certfile and ssl_keyfile the public port speaks HTTPS to the
// browser, and the router still reaches the plain Gradio behind it.
func TestRouterServesTLSInFrontOfPlainGradio(t *testing.T) {
	certFile, keyFile := writeCert(t, "")
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tlsListener(raw, appenv.SSL{CertFile: certFile, KeyFile: keyFile})
	if err != nil {
		t.Fatal(err)
	}
	serveRouterOver(t, ln)

	pool := x509.NewCertPool()
	pem, _ := os.ReadFile(certFile)
	pool.AppendCertsFromPEM(pem)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
	resp, err := client.Get("https://" + raw.Addr().String() + "/")
	if err != nil {
		t.Fatalf("https GET (certificate verified against the file): %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "gradio says https" {
		t.Errorf("GET = %d %q, want 200 \"gradio says https\"", resp.StatusCode, body)
	}
	// Plain HTTP on the TLS port must not be answered as a page.
	if r, err := (&http.Client{Timeout: 3 * time.Second}).Get("http://" + raw.Addr().String() + "/"); err == nil {
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if r.StatusCode == 200 && strings.Contains(string(b), "gradio says") {
			t.Errorf("plain http reached Gradio through the TLS port: %q", b)
		}
	}
}

func TestTLSListenerWithoutSSLOptionsIsPlain(t *testing.T) {
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	ln, err := tlsListener(raw, appenv.SSL{})
	if err != nil || ln != raw {
		t.Fatalf("tlsListener without options = %v, %v; want the same listener", ln, err)
	}
}

func TestTLSListenerRefusesHalfAndBadFiles(t *testing.T) {
	certFile, keyFile := writeCert(t, "")
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	for name, ssl := range map[string]appenv.SSL{
		"cert only":    {CertFile: certFile},
		"key only":     {KeyFile: keyFile},
		"missing cert": {CertFile: filepath.Join(t.TempDir(), "nope.pem"), KeyFile: keyFile},
		"swapped":      {CertFile: keyFile, KeyFile: certFile},
	} {
		if _, err := tlsListener(raw, ssl); err == nil {
			t.Errorf("%s: want an error, got none", name)
		}
	}
}

func TestTLSListenerReadsAPasswordProtectedKey(t *testing.T) {
	certFile, keyFile := writeCert(t, "s3cret-pass")
	for password, ok := range map[string]bool{"s3cret-pass": true, "wrong": false, "": false} {
		raw, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, err = tlsListener(raw, appenv.SSL{CertFile: certFile, KeyFile: keyFile, KeyPassword: password})
		_ = raw.Close()
		if (err == nil) != ok {
			t.Errorf("password %q: err = %v, want ok=%v", password, err, ok)
		}
		if err != nil && strings.Contains(err.Error(), "s3cret-pass") {
			t.Errorf("the error prints the password: %v", err)
		}
	}
}
