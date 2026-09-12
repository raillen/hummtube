package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("chave vazia")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	derived := pbkdf2.Key(key, salt, 100000, 32, sha256.New)
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	out := append(salt, nonce...)
	out = append(out, ct...)
	return out, nil
}

func Decrypt(key, data []byte) ([]byte, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("dados muito curtos")
	}
	salt := data[:16]
	derived := pbkdf2.Key(key, salt, 100000, 32, sha256.New)
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < 16+nonceSize {
		return nil, fmt.Errorf("dados muito curtos")
	}
	nonce := data[16 : 16+nonceSize]
	ct := data[16+nonceSize:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("senha incorreta ou dados adulterados")
	}
	return plain, nil
}
