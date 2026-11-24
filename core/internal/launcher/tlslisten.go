package launcher

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
)

// tlsListener makes the router's public listener serve HTTPS when config.json
// names a certificate and key (ssl_certfile, ssl_keyfile). Gradio itself stays
// plain HTTP behind the router (launchopts hands it no ssl_ options), so the
// router is the only place the certificate is read. Without ssl_ options it
// returns ln unchanged.
func tlsListener(ln net.Listener, ssl appenv.SSL) (net.Listener, error) {
	if ssl.CertFile == "" && ssl.KeyFile == "" {
		return ln, nil
	}
	if !ssl.Enabled() {
		return nil, errors.New("config.json: ssl_certfile and ssl_keyfile must both be set to serve HTTPS")
	}
	cert, err := loadKeyPair(ssl)
	if err != nil {
		return nil, err
	}
	return tls.NewListener(ln, &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		// Browsers speak h2, but the router proxies WebSockets and SSE as HTTP/1.1.
		NextProtos: []string{"http/1.1"},
	}), nil
}

// loadKeyPair reads the certificate and key, decrypting a key that PEM
// encrypted with ssl_keyfile_password (the legacy "Proc-Type: 4,ENCRYPTED" form
// openssl writes by default for `-des3`; PKCS#8 encrypted keys are not read).
func loadKeyPair(ssl appenv.SSL) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(ssl.CertFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("ssl_certfile: %w", err)
	}
	keyPEM, err := os.ReadFile(ssl.KeyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("ssl_keyfile: %w", err)
	}
	if ssl.KeyPassword != "" {
		if keyPEM, err = decryptKey(keyPEM, ssl.KeyPassword); err != nil {
			return tls.Certificate{}, err
		}
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("ssl_certfile and ssl_keyfile: %w", err)
	}
	return cert, nil
}

func decryptKey(keyPEM []byte, password string) ([]byte, error) {
	var out []byte
	rest := keyPEM
	for {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		if b.Type == "ENCRYPTED PRIVATE KEY" {
			return nil, errors.New("ssl_keyfile: a PKCS#8 encrypted key is not supported; write the key without a password (openssl pkey -in key.pem -out key.plain.pem) or as a legacy PEM (openssl rsa -traditional)")
		}
		//lint:ignore SA1019 the legacy PEM encryption is the only one a password-protected ssl_keyfile uses here
		if x509.IsEncryptedPEMBlock(b) {
			der, err := x509.DecryptPEMBlock(b, []byte(password))
			if err != nil {
				return nil, fmt.Errorf("ssl_keyfile_password: %w", err)
			}
			b = &pem.Block{Type: b.Type, Bytes: der}
		}
		out = append(out, pem.EncodeToMemory(b)...)
	}
	return out, nil
}
