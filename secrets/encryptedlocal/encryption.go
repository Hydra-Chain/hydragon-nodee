package encryptedlocal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"

	"golang.org/x/crypto/scrypt"
)

type PasswordHandler interface {
	GeneratePassword() ([]byte, error)
	InputPassword(bool) ([]byte, error)
}

type Encryption interface {
	Encrypt(data []byte, pwd []byte) ([]byte, error)
	Decrypt(data []byte, pwd []byte) ([]byte, error)
}

type encryption struct {
}

func NewEncryption() Encryption {
	return &encryption{}
}

func (ch *encryption) Encrypt(data []byte, pwd []byte) ([]byte, error) {
	key, salt, err := DeriveKey(pwd, nil)
	if err != nil {
		return nil, err
	}

	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	cipherText := gcm.Seal(nonce, nonce, data, nil)

	return append(cipherText, salt...), nil
}

func (ch *encryption) Decrypt(data []byte, pwd []byte) ([]byte, error) {
	salt, data := data[len(data)-32:], data[:len(data)-32]

	key, _, err := DeriveKey(pwd, salt)
	if err != nil {
		return nil, err
	}

	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, err
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func DeriveKey(password, salt []byte) ([]byte, []byte, error) {
	if salt == nil {
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, nil, err
		}
	}

	key, err := scrypt.Key(password, salt, 1048576, 8, 1, 32)
	if err != nil {
		return nil, nil, err
	}

	return key, salt, nil
}
