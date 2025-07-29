package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"

	"golang.org/x/crypto/nacl/box"
)

type EncryptedPayload struct {
	Ciphertext      string `json:"ciphertext"`
	Nonce           string `json:"nonce"`
	SignalPublicKey string `json:"signalPublicKey"`
}

// EncryptMessage encrypts a message using NaCl box
func EncryptMessage(message interface{}, signalPrivateKeyB64, signalPublicKeyB64, appPublicKeyB64 string) (*EncryptedPayload, error) {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	signalPrivKeyBytes, err := base64.StdEncoding.DecodeString(signalPrivateKeyB64)
	if err != nil {
		return nil, err
	}
	signalPubKeyBytes, err := base64.StdEncoding.DecodeString(signalPublicKeyB64)
	if err != nil {
		return nil, err
	}
	appPubKeyBytes, err := base64.StdEncoding.DecodeString(appPublicKeyB64)
	if err != nil {
		return nil, err
	}

	var signalPrivKey [32]byte
	var signalPubKey [32]byte
	var appPubKey [32]byte
	copy(signalPrivKey[:], signalPrivKeyBytes)
	copy(signalPubKey[:], signalPubKeyBytes)
	copy(appPubKey[:], appPubKeyBytes)

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}

	ciphertext := box.Seal(nil, msgBytes, &nonce, &appPubKey, &signalPrivKey)

	return &EncryptedPayload{
		Ciphertext:      base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:           base64.StdEncoding.EncodeToString(nonce[:]),
		SignalPublicKey: base64.StdEncoding.EncodeToString(signalPubKey[:]),
	}, nil
}

// DecryptMessage decrypts a message using NaCl box
func DecryptMessage(payload EncryptedPayload, signalPrivateKeyB64, appPublicKeyB64 string) (map[string]interface{}, error) {
	// Decode base64 fields
	cipherBytes, err := base64.StdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, err
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return nil, err
	}

	appPubKeyBytes, err := base64.StdEncoding.DecodeString(appPublicKeyB64)
	if err != nil {
		return nil, err
	}

	signalPrivKeyBytes, err := base64.StdEncoding.DecodeString(signalPrivateKeyB64)
	if err != nil {
		return nil, err
	}

	// Prepare key arrays
	var nonce [24]byte
	var appPubKey [32]byte
	var signalPrivKey [32]byte

	copy(nonce[:], nonceBytes)
	copy(appPubKey[:], appPubKeyBytes)
	copy(signalPrivKey[:], signalPrivKeyBytes)

	// Decrypt using NaCl box
	decrypted, ok := box.Open(nil, cipherBytes, &nonce, &appPubKey, &signalPrivKey)
	if !ok {
		return nil, errors.New("failed to decrypt message")
	}

	// Unmarshal decrypted JSON
	var result map[string]interface{}
	err = json.Unmarshal(decrypted, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
