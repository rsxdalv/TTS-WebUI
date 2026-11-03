package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

func main() {
	// Throwaway test seed — NOT a real key.
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}

	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)

	keyID := [8]byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01, 0x02, 0x03}

	pubRaw := make([]byte, 42)
	pubRaw[0] = 'E'
	pubRaw[1] = 'd'
	copy(pubRaw[2:10], keyID[:])
	copy(pubRaw[10:42], pubKey)

	fmt.Printf("untrusted comment: minisign public key tts-webui-test\n%s\n",
		base64.StdEncoding.EncodeToString(pubRaw))
	fmt.Printf("\nKeyID hex: %x\n", keyID)
	fmt.Printf("PubKey hex: %x\n", pubKey)
}
