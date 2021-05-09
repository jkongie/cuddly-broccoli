package wsrpc

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"math/big"
)

// Generates a minimal certificate (that wouldn't be considered valid outside this telemetry networking protocol)
// from an Ed25519 private key.
func newMinimalX509CertFromPrivateKey(sk ed25519.PrivateKey) tls.Certificate {
	template := x509.Certificate{
		SerialNumber: big.NewInt(0), // serial number must be set, so we set it to 0
	}

	encodedCert, err := x509.CreateCertificate(rand.Reader, &template, &template, sk.Public(), sk)
	if err != nil {
		panic(err)
	}

	// Uncomment this if you want to get an encoded cert you can feed into openssl x509 etc...
	//
	// var buf bytes.Buffer
	// if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: encodedCert}); err != nil {
	// 	log.Fatalf("Failed to encode cert into pem format: %v", err)
	// }
	// fmt.Printf("pubkey: %x\nencodedCert: %v\n", sk.Public(), buf.String())

	return tls.Certificate{
		Certificate: [][]byte{encodedCert},

		PrivateKey:                   sk,
		SupportedSignatureAlgorithms: []tls.SignatureScheme{tls.Ed25519},
	}
}

func verifyCertMatchesIdentity(identities map[[ed25519.PublicKeySize]byte]string) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) != 1 {
			return fmt.Errorf("Required exactly one client certificate")
		}
		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return err
		}
		pk, err := pubKeyFromCert(cert)
		if err != nil {
			return err
		}

		fmt.Println(pk)

		name, ok := identities[pk]
		if !ok {
			return fmt.Errorf("Unknown public key on cert %x", pk)
		}

		log.Printf("Got good cert from %v", name)
		return nil
	}
}

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
