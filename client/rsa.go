package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"encoding/gob"
	"encoding/pem"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

// GenerateRSAKey create 2 file, the private and public key
func GenerateRSAKey(myUsername string) {
	reader := rand.Reader
	bitSize := 2048

	key, err := rsa.GenerateKey(reader, bitSize)
	checkError(err)

	publicKey := key.PublicKey

	saveGobKey("./client/RsaKeys" + myUsername + "/private.key", key)
	savePEMKey("./client/RsaKeys" + myUsername + "/private.pem", key)

	saveGobKey("./client/RsaKeys" + myUsername + "/public.key", publicKey)
	savePublicPEMKey("./client/RsaKeys" + myUsername + "/public.pem", publicKey)
}

func saveGobKey(fileName string, key interface{}) {
	outFile, err := os.Create(fileName)
	checkError(err)
	defer outFile.Close()

	encoder := gob.NewEncoder(outFile)
	err = encoder.Encode(key)
	checkError(err)
}

func savePEMKey(fileName string, key *rsa.PrivateKey) {
	outFile, err := os.Create(fileName)
	checkError(err)
	defer outFile.Close()

	var privateKey = &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	err = pem.Encode(outFile, privateKey)
	checkError(err)
}

func savePublicPEMKey(fileName string, pubkey rsa.PublicKey) {
	asn1Bytes, err := asn1.Marshal(pubkey)
	checkError(err)

	var pemkey = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: asn1Bytes,
	}

	pemfile, err := os.Create(fileName)
	checkError(err)
	defer pemfile.Close()

	err = pem.Encode(pemfile, pemkey)
	checkError(err)
}

func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
		os.Exit(1)
	}
}

// BytesToPrivateKey bytes to private key
func BytesToPrivateKey(priv string) *rsa.PrivateKey {
	block, _ := pem.Decode([]byte(priv))
	if block == nil {
		log.Error("failed to parse PEM block containing the public key")
	}
	enc := x509.IsEncryptedPEMBlock(block)
	b := block.Bytes
	var err error
	if enc {
		log.Println("is encrypted pem block")
		b, err = x509.DecryptPEMBlock(block, nil)
		if err != nil {
			log.Error(err)
		}
	}
	key, err := x509.ParsePKCS1PrivateKey(b)
	if err != nil {
		log.Error(err)
	}
	return key

	// var pk rsa.PrivateKey
	// asn1.Unmarshal(block.Bytes, &pk)
	// return &pk
}

// BytesToPublicKey bytes to public key
func BytesToPublicKey(pub string) *rsa.PublicKey {
	block, _ := pem.Decode([]byte(pub))
	if block == nil {
		log.Error("failed to parse PEM block containing the public key")
	}

	var pk rsa.PublicKey
	asn1.Unmarshal(block.Bytes, &pk)

	return &pk

	// block, _ := pem.Decode([]byte(pub))
	// log.Info(block)
	// // enc := x509.IsEncryptedPEMBlock(block)
	// b := block.Bytes
	// // var err error
	// // if enc {
	// // 	log.Println("is encrypted pem block")
	// // 	b, err = x509.DecryptPEMBlock(block, nil)
	// // 	if err != nil {
	// // 		log.Error(err)
	// // 	}
	// // }
	// log.Info(b)
	// ifc, err := x509.ParsePKIXPublicKey(b)
	// if err != nil {
	// 	log.Error(err)
	// }
	// key, ok := ifc.(*rsa.PublicKey)
	// if !ok {
	// 	log.Error("not ok")
	// }
	// return key
}

// EncryptWithPublicKey encrypts data with public key
func EncryptWithPublicKey(msg []byte, pub *rsa.PublicKey) []byte {
	hash := sha512.New()
	ciphertext, err := rsa.EncryptOAEP(hash, rand.Reader, pub, msg, nil)
	if err != nil {
		log.Error(err)
	}
	return ciphertext
}

// DecryptWithPrivateKey decrypts data with private key
func DecryptWithPrivateKey(ciphertext []byte, priv *rsa.PrivateKey) []byte {
	hash := sha512.New()
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, priv, ciphertext, nil)
	if err != nil {
		log.Error(err)
	}
	return plaintext
}
