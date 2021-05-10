package wsrpc

import (
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
)

func pubKeyFromCert(cert *x509.Certificate) (pk [ed25519.PublicKeySize]byte, err error) {
	if cert.PublicKeyAlgorithm != x509.Ed25519 {
		return pk, fmt.Errorf("Require ed25519 public key")
	}
	return StaticallySizedEd25519PublicKey(cert.PublicKey)
}

func StaticallySizedEd25519PublicKey(publickey crypto.PublicKey) ([ed25519.PublicKeySize]byte, error) {
	var result [ed25519.PublicKeySize]byte

	pkslice, ok := publickey.(ed25519.PublicKey)
	if !ok {
		return result, fmt.Errorf("Invalid ed25519 public key")
	}
	if ed25519.PublicKeySize != copy(result[:], pkslice) {
		// assertion
		panic("copying public key failed")
	}
	return result, nil
}
