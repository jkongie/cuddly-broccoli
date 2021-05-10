package wsrpc

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"math/big"
)

func NewServerTLSConfig(priv ed25519.PrivateKey, clientIdentities map[[32]byte]string) tls.Config {
	cert := newMinimalX509CertFromPrivateKey(priv)

	return tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAnyClientCert,

		// Since our clients use self-signed certs, we skip verficiation here.
		// Instead, we use VerifyPeerCertificate for our own check
		InsecureSkipVerify: true,

		MaxVersion: tls.VersionTLS13,
		MinVersion: tls.VersionTLS13,

		VerifyPeerCertificate: verifyCertMatchesIdentity(clientIdentities),
	}
}

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

	return tls.Certificate{
		Certificate:                  [][]byte{encodedCert},
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

		name, ok := identities[pk]
		if !ok {
			return fmt.Errorf("Unknown public key on cert %x", pk)
		}

		log.Printf("Got good cert from %v", name)
		return nil
	}
}
