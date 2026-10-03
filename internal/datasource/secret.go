package datasource

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

// seal encrypts a secret with AES-256-GCM under key, with a random nonce
// and the data source ID as additional data, so that a value cannot be
// moved to another data source.
func seal(key []byte, id, plain string) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nil, []byte(plain), []byte(id)), nil
}

// unseal decrypts a value sealed by seal.
func unseal(key []byte, id string, sealed []byte) (string, error) {
	if key == nil {
		return "", errors.New("no secret key")
	}

	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}

	plain, err := aead.Open(nil, nil, sealed, []byte(id))
	return string(plain), err
}

// newAEAD returns AES-GCM that prepends a random nonce to each sealed value.
func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithRandomNonce(block)
}
