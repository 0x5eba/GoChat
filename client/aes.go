package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"log"
)

func CreateKey() ([]byte, []byte, error) {
	genkey := make([]byte, 16)
	_, err := rand.Read(genkey)
	if err != nil {
		log.Fatalf("Failed to read new random key: %s", err)
	}

	IV := make([]byte, 16)
	_, err = rand.Read(IV)
	if err != nil {
		log.Fatalf("Failed to read new random key: %s", err)
	}
	return genkey, IV, err
}

func createCipher(key []byte) cipher.Block {
	c, err := aes.NewCipher(key)
	if err != nil {
		log.Fatalf("Failed to create the AES cipher: %s", err)
	}
	return c
}

func EncryptAes(plainText []byte, key, IV []byte) []byte {
	bytes := plainText
	blockCipher := createCipher(key)
	stream := cipher.NewCTR(blockCipher, IV)
	stream.XORKeyStream(bytes, bytes)
	return bytes
}

func DecryptAes(chiperText, key, IV []byte) []byte {
	bytes := chiperText
	blockCipher := createCipher(key)
	stream := cipher.NewCTR(blockCipher, IV)
	stream.XORKeyStream(bytes, bytes)
	return bytes
}
