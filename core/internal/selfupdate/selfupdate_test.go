package selfupdate

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"testing"
)

// testSeed is the deterministic seed for the throwaway test key.
// Matches testdata/test.pub — NOT a real key, never use in production.
var testSeed = func() []byte {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	return seed
}()

var testKeyID = [8]byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01, 0x02, 0x03}

func testPrivKey() ed25519.PrivateKey { return ed25519.NewKeyFromSeed(testSeed) }
func testPubKeyPEM() string           { return embeddedPublicKey }

func TestParsePubKey(t *testing.T) {
	k, err := parsePubKey([]byte(embeddedPublicKey))
	if err != nil {
		t.Fatalf("parsePubKey: %v", err)
	}
	if k.keyID != testKeyID {
		t.Errorf("keyID=%x, want %x", k.keyID, testKeyID)
	}
	if len(k.pubKey) != 32 {
		t.Errorf("pubKey length=%d, want 32", len(k.pubKey))
	}
}

func TestParsePubKeyFromFile(t *testing.T) {
	data, err := os.ReadFile("testdata/test.pub")
	if err != nil {
		t.Fatalf("reading testdata/test.pub: %v", err)
	}
	k, err := parsePubKey(data)
	if err != nil {
		t.Fatalf("parsePubKey from file: %v", err)
	}
	if k.keyID != testKeyID {
		t.Errorf("keyID from file=%x, want %x", k.keyID, testKeyID)
	}
}

func TestVerifyMinisign_Valid(t *testing.T) {
	msg := []byte("hello tts-webui update payload")
	sk := testPrivKey()
	sigText := signMinisign(sk, testKeyID, msg, "timestamp:2026")

	pubKey, err := parsePubKey([]byte(embeddedPublicKey))
	if err != nil {
		t.Fatalf("parsePubKey: %v", err)
	}
	sig, err := parseSig([]byte(sigText))
	if err != nil {
		t.Fatalf("parseSig: %v", err)
	}
	if err := verifyMinisign(pubKey, sig, msg); err != nil {
		t.Fatalf("verifyMinisign: %v", err)
	}
}

func TestVerifyMinisign_Tampered(t *testing.T) {
	msg := []byte("original payload")
	sk := testPrivKey()
	sigText := signMinisign(sk, testKeyID, msg, "ts:1")

	pubKey, _ := parsePubKey([]byte(embeddedPublicKey))
	sig, _ := parseSig([]byte(sigText))

	tampered := []byte("tampered payload")
	if err := verifyMinisign(pubKey, sig, tampered); err == nil {
		t.Error("expected verification failure for tampered payload")
	}
}

func TestVerifyMinisign_KeyIDMismatch(t *testing.T) {
	msg := []byte("payload")
	sk := testPrivKey()
	wrongKeyID := [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	sigText := signMinisign(sk, wrongKeyID, msg, "ts:1")

	pubKey, _ := parsePubKey([]byte(embeddedPublicKey))
	sig, _ := parseSig([]byte(sigText))

	if err := verifyMinisign(pubKey, sig, msg); err == nil {
		t.Error("expected verification failure for mismatched key ID")
	}
}

func TestParsePubKey_BadBase64(t *testing.T) {
	bad := "untrusted comment: x\n!!not-base64!!\n"
	if _, err := parsePubKey([]byte(bad)); err == nil {
		t.Error("expected error for bad base64 in public key")
	}
}

func TestParseSig_TooFewLines(t *testing.T) {
	if _, err := parseSig([]byte("only one line")); err == nil {
		t.Error("expected error for signature with too few lines")
	}
}

func TestBinaryAssetName(t *testing.T) {
	name := binaryAssetName()
	if name == "" {
		t.Error("binaryAssetName returned empty string")
	}
}

func TestEmbeddedPublicKeyMatchesTestdataFile(t *testing.T) {
	fileData, err := os.ReadFile("testdata/test.pub")
	if err != nil {
		t.Fatalf("reading testdata/test.pub: %v", err)
	}
	// Both should decode to the same raw bytes.
	embeddedLine2 := extractLine2(embeddedPublicKey)
	fileLine2 := extractLine2(string(fileData))
	if embeddedLine2 != fileLine2 {
		t.Errorf("embedded key differs from testdata/test.pub\nembedded: %q\nfile:     %q",
			embeddedLine2, fileLine2)
	}
}

func extractLine2(s string) string {
	lines := splitLines(s)
	if len(lines) >= 2 {
		return lines[1]
	}
	return ""
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// roundtrip encode/decode sanity check
func TestBase64RoundTrip(t *testing.T) {
	raw := make([]byte, 42)
	raw[0] = 'E'
	raw[1] = 'd'
	encoded := base64.StdEncoding.EncodeToString(raw)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != string(raw) {
		t.Error("base64 round-trip mismatch")
	}
}
