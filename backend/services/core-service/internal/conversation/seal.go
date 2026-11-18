package conversation

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
)

// KeySize is the length of the message encryption key (AES-256).
const KeySize = 32

// sealer stores nonce || AES-256-GCM ciphertext || tag, bound to conversation, message and role.
type sealer struct {
	aead cipher.AEAD
}

func newSealer(key []byte) (sealer, error) {
	if len(key) != KeySize {
		return sealer{}, errors.New("message key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return sealer{}, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return sealer{}, err
	}

	return sealer{aead: aead}, nil
}

func messageAAD(conversationID, messageID, role string) []byte {
	b, _ := json.Marshal([]string{"govai-message-v1", conversationID, messageID, role})
	return b
}

func (s sealer) seal(text, conversationID, messageID, role string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return s.aead.Seal(nonce, nonce, []byte(text), messageAAD(conversationID, messageID, role)), nil
}

func (s sealer) open(sealed []byte, conversationID, messageID, role string) (string, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n+s.aead.Overhead() {
		return "", errors.New("sealed message too short")
	}

	plain, err := s.aead.Open(nil, sealed[:n], sealed[n:], messageAAD(conversationID, messageID, role))
	if err != nil {
		return "", err
	}

	return string(plain), nil
}
