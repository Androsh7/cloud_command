package sshx

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

const (
	PrivateKeyFileName = "private_rsa.key"
	PublicKeyFileName  = "public_rsa.key"

	rsaKeyBits            = 2048
	privateKeyPermissions = 0o600
	publicKeyPermissions  = 0o644
	privateKeyPemType     = "RSA PRIVATE KEY"
)

// KeyPair holds the paths of a private key and its matching OpenSSH public key.
type KeyPair struct {
	PrivateKeyPath string
	PublicKeyPath  string
}

// GenerateRsaKeyPair writes an unencrypted PEM private key and an OpenSSH public key into a directory.
func GenerateRsaKeyPair(directory string) (KeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return KeyPair{}, fmt.Errorf("generating rsa key: %w", err)
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return KeyPair{}, fmt.Errorf("converting public key: %w", err)
	}

	keyPair := KeyPair{
		PrivateKeyPath: filepath.Join(directory, PrivateKeyFileName),
		PublicKeyPath:  filepath.Join(directory, PublicKeyFileName),
	}
	privateKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  privateKeyPemType,
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err := os.WriteFile(keyPair.PrivateKeyPath, privateKeyPem, privateKeyPermissions); err != nil {
		return KeyPair{}, fmt.Errorf("writing private key %q: %w", keyPair.PrivateKeyPath, err)
	}
	if err := os.WriteFile(keyPair.PublicKeyPath, ssh.MarshalAuthorizedKey(publicKey), publicKeyPermissions); err != nil {
		return KeyPair{}, fmt.Errorf("writing public key %q: %w", keyPair.PublicKeyPath, err)
	}
	return keyPair, nil
}
