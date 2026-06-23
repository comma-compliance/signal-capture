package utils

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/nacl/box"
)

// genKeyB64 returns a freshly generated NaCl box keypair, base64-encoded the
// same way the production code expects its key inputs.
func genKeyB64(t *testing.T) (pubB64, privB64 string) {
	t.Helper()
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("box.GenerateKey: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pub[:]), base64.StdEncoding.EncodeToString(priv[:])
}

// TestEncryptMessage_DecryptMessage_RoundTrip locks the observable contract of
// the NaCl box encrypt/decrypt seam: a message sealed by EncryptMessage is
// recovered intact by DecryptMessage, and the emitted payload carries a 24-byte
// nonce plus the signal public key. This is the contract that must survive a
// golang.org/x/crypto bump.
func TestEncryptMessage_DecryptMessage_RoundTrip(t *testing.T) {
	signalPubB64, signalPrivB64 := genKeyB64(t)
	appPubB64, _ := genKeyB64(t)

	msg := map[string]interface{}{
		"body":   "hello signal",
		"source": "+15551230000",
	}

	payload, err := EncryptMessage(msg, signalPrivB64, signalPubB64, appPubB64)
	if err != nil {
		t.Fatalf("EncryptMessage: %v", err)
	}

	if payload.Ciphertext == "" {
		t.Error("expected a non-empty ciphertext")
	}
	if payload.SignalPublicKey != signalPubB64 {
		t.Errorf("SignalPublicKey = %q, want %q", payload.SignalPublicKey, signalPubB64)
	}
	nonce, err := base64.StdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		t.Fatalf("nonce is not valid base64: %v", err)
	}
	if len(nonce) != 24 {
		t.Errorf("nonce length = %d, want 24", len(nonce))
	}

	got, err := DecryptMessage(*payload, signalPrivB64, appPubB64)
	if err != nil {
		t.Fatalf("DecryptMessage: %v", err)
	}
	if got["body"] != "hello signal" {
		t.Errorf("decrypted body = %v, want %q", got["body"], "hello signal")
	}
	if got["source"] != "+15551230000" {
		t.Errorf("decrypted source = %v, want %q", got["source"], "+15551230000")
	}
}

// TestDecryptMessage_TamperedCiphertextFails confirms the authenticated-
// encryption contract: flipping a ciphertext byte must make decryption fail
// rather than return garbage.
func TestDecryptMessage_TamperedCiphertextFails(t *testing.T) {
	signalPubB64, signalPrivB64 := genKeyB64(t)
	appPubB64, _ := genKeyB64(t)

	payload, err := EncryptMessage(map[string]interface{}{"body": "secret"}, signalPrivB64, signalPubB64, appPubB64)
	if err != nil {
		t.Fatalf("EncryptMessage: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		t.Fatalf("ciphertext not base64: %v", err)
	}
	raw[0] ^= 0xff
	payload.Ciphertext = base64.StdEncoding.EncodeToString(raw)

	if _, err := DecryptMessage(*payload, signalPrivB64, appPubB64); err == nil {
		t.Error("expected decryption of tampered ciphertext to fail, got nil error")
	}
}

// TestDecryptMessage_WrongKeyFails confirms decryption fails when the private
// key does not match the one used to seal the payload.
func TestDecryptMessage_WrongKeyFails(t *testing.T) {
	signalPubB64, signalPrivB64 := genKeyB64(t)
	appPubB64, _ := genKeyB64(t)

	payload, err := EncryptMessage(map[string]interface{}{"body": "secret"}, signalPrivB64, signalPubB64, appPubB64)
	if err != nil {
		t.Fatalf("EncryptMessage: %v", err)
	}

	_, otherPrivB64 := genKeyB64(t)
	if _, err := DecryptMessage(*payload, otherPrivB64, appPubB64); err == nil {
		t.Error("expected decryption with a mismatched private key to fail, got nil error")
	}
}

// TestEncryptMessage_InvalidKeyEncoding confirms invalid base64 key material is
// reported as an error rather than panicking.
func TestEncryptMessage_InvalidKeyEncoding(t *testing.T) {
	if _, err := EncryptMessage(map[string]interface{}{"a": 1}, "not!base64", "also!bad", "still!bad"); err == nil {
		t.Error("expected an error for invalid base64 key material, got nil")
	}
}
