package selfupdate

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// minisignPubKey is a parsed minisign public key.
type minisignPubKey struct {
	keyID  [8]byte
	pubKey ed25519.PublicKey
}

// minisignSig is a parsed minisign signature.
type minisignSig struct {
	keyID          [8]byte
	sig            [64]byte
	trustedComment string
	globalSig      [64]byte
}

// parsePubKey parses a two-line minisign public key file.
//
// Format:
//
//	untrusted comment: <ignored>
//	<base64(algorithm[2] + keyid[8] + pubkey[32])>
func parsePubKey(data []byte) (minisignPubKey, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return minisignPubKey{}, errors.New("invalid public key: expected 2 lines")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil {
		return minisignPubKey{}, fmt.Errorf("decoding public key: %w", err)
	}
	if len(raw) != 42 { // 2 + 8 + 32
		return minisignPubKey{}, fmt.Errorf("invalid public key length: %d (want 42)", len(raw))
	}
	if raw[0] != 'E' || raw[1] != 'd' {
		return minisignPubKey{}, fmt.Errorf("unsupported algorithm: %q", string(raw[:2]))
	}
	var k minisignPubKey
	copy(k.keyID[:], raw[2:10])
	k.pubKey = make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(k.pubKey, raw[10:42])
	return k, nil
}

// parseSig parses a four-line minisign signature file.
//
// Format:
//
//	untrusted comment: <ignored>
//	<base64(algorithm[2] + keyid[8] + sig[64])>
//	trusted comment: <trusted_comment_text>
//	<base64(global_sig[64])>
func parseSig(data []byte) (minisignSig, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 4 {
		return minisignSig{}, fmt.Errorf("invalid signature: expected 4 lines, got %d", len(lines))
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil {
		return minisignSig{}, fmt.Errorf("decoding signature: %w", err)
	}
	if len(raw) != 74 { // 2 + 8 + 64
		return minisignSig{}, fmt.Errorf("invalid signature length: %d (want 74)", len(raw))
	}
	if raw[0] != 'E' || raw[1] != 'd' {
		return minisignSig{}, fmt.Errorf("unsupported signature algorithm: %q", string(raw[:2]))
	}

	var s minisignSig
	copy(s.keyID[:], raw[2:10])
	copy(s.sig[:], raw[10:74])

	if !strings.HasPrefix(lines[2], "trusted comment: ") {
		return minisignSig{}, fmt.Errorf("invalid trusted comment line: %q", lines[2])
	}
	s.trustedComment = strings.TrimPrefix(lines[2], "trusted comment: ")

	graw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[3]))
	if err != nil {
		return minisignSig{}, fmt.Errorf("decoding global signature: %w", err)
	}
	if len(graw) != 64 {
		return minisignSig{}, fmt.Errorf("invalid global signature length: %d (want 64)", len(graw))
	}
	copy(s.globalSig[:], graw)

	return s, nil
}

// verifyMinisign verifies the minisign signature of msg using key.
// Checks both the content signature and the global (trusted-comment) signature.
func verifyMinisign(key minisignPubKey, sig minisignSig, msg []byte) error {
	if key.keyID != sig.keyID {
		return fmt.Errorf("key ID mismatch: pubkey=%x sig=%x", key.keyID, sig.keyID)
	}
	if !ed25519.Verify(key.pubKey, msg, sig.sig[:]) {
		return errors.New("content signature verification failed")
	}
	// Global signature covers: sig_bytes || trusted_comment_text
	globalMsg := append(sig.sig[:64:64], []byte(sig.trustedComment)...)
	if !ed25519.Verify(key.pubKey, globalMsg, sig.globalSig[:]) {
		return errors.New("global signature verification failed")
	}
	return nil
}

// signMinisign signs msg with sk, returning a four-line minisign signature file.
// For tests only — the private key is a throwaway test key.
func signMinisign(sk ed25519.PrivateKey, keyID [8]byte, msg []byte, trustedComment string) string {
	sig := ed25519.Sign(sk, msg)

	globalMsg := append(sig[:64:64], []byte(trustedComment)...)
	globalSig := ed25519.Sign(sk, globalMsg)

	header := make([]byte, 74)
	header[0] = 'E'
	header[1] = 'd'
	copy(header[2:10], keyID[:])
	copy(header[10:74], sig)

	var sb strings.Builder
	sb.WriteString("untrusted comment: signature from tts-webui test key\n")
	sb.WriteString(base64.StdEncoding.EncodeToString(header) + "\n")
	sb.WriteString("trusted comment: " + trustedComment + "\n")
	sb.WriteString(base64.StdEncoding.EncodeToString(globalSig) + "\n")
	return sb.String()
}
