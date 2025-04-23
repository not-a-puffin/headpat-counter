package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

const sessionLifetime time.Duration = 24 * 60 * time.Hour

type Session struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
	UserId  string `json:"user_id"`
}

type SessionStore interface {
	SetSession(token string, session Session) error
	GetSession(token string) (*Session, error)
	DeleteSession(token string) error
	ContainsSession(token string) bool
}

func getEncryptionKey() ([]byte, error) {
	keyStr := os.Getenv("ENCRYPTION_KEY")

	if len(keyStr) == 0 {
		return nil, errors.New("missing encryption key")
	}

	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, err
	}

	if len(key) != 32 {
		return nil, errors.New("invalid encryption key")
	}

	return key, nil
}

func encrypt(bytes []byte) ([]byte, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesgcm.Seal(nil, nonce, bytes, nil)
	return append(nonce, ciphertext...), nil
}

func decrypt(bytes []byte) ([]byte, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesgcm.NonceSize()
	if len(bytes) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := bytes[:nonceSize]
	ciphertext := bytes[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func encodeSession(session Session) (string, error) {
	jsonBytes, err := json.Marshal(session)
	if err != nil {
		return "", err
	}

	encrypted, err := encrypt(jsonBytes)
	if err != nil {
		return "", err
	}

	base64Str := base64.StdEncoding.EncodeToString(encrypted)
	return base64Str, nil
}

func decodeSession(base64Str string) (Session, error) {
	var session Session

	bytes, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return session, err
	}

	decrypted, err := decrypt(bytes)
	if err != nil {
		return session, err
	}

	err = json.Unmarshal(decrypted, &session)
	return session, err
}
