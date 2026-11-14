package sshx

import (
	"os"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGenerateRsaKeyPairProducesMatchingKeys(t *testing.T) {
	keyPair, err := GenerateRsaKeyPair(t.TempDir())
	if err != nil {
		t.Fatalf("GenerateRsaKeyPair returned error: %v", err)
	}

	privateKeyBytes, err := os.ReadFile(keyPair.PrivateKeyPath)
	if err != nil {
		t.Fatalf("reading private key: %v", err)
	}
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		t.Fatalf("parsing private key: %v", err)
	}
	publicKeyBytes, err := os.ReadFile(keyPair.PublicKeyPath)
	if err != nil {
		t.Fatalf("reading public key: %v", err)
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(publicKeyBytes)
	if err != nil {
		t.Fatalf("parsing public key: %v", err)
	}

	if ssh.FingerprintSHA256(signer.PublicKey()) != ssh.FingerprintSHA256(publicKey) {
		t.Fatal("public key does not match private key")
	}
}
