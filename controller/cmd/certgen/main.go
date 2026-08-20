package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	root := env("CERT_OUTPUT_DIR", "/certs")
	required := []string{"ca.crt", "controller.crt", "controller.key", "agent.crt", "agent.key"}
	complete := true
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			complete = false
		}
	}
	if complete {
		log.Printf("certificate bundle already exists")
		return
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		log.Fatal(err)
	}
	if err := os.Chmod(root, 0755); err != nil {
		log.Fatal(err)
	}
	now := time.Now().UTC()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	caTemplate := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "MyPanel Local CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(5, 0, 0), IsCA: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		log.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		log.Fatal(err)
	}
	writePEM(filepath.Join(root, "ca.crt"), "CERTIFICATE", caDER, 0644)
	issue(root, "controller", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, nil, nil, caCert, caKey, now)
	issue(root, "agent", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, []string{"agent", "localhost"}, []net.IP{net.ParseIP("127.0.0.1")}, caCert, caKey, now)
	for _, name := range []string{"ca.crt", "controller.crt", "controller.key"} {
		if err := os.Chown(filepath.Join(root, name), 65532, 65532); err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("generated MyPanel mTLS bundle")
}

func issue(root, name string, usages []x509.ExtKeyUsage, dns []string, ips []net.IP, ca *x509.Certificate, caKey *ecdsa.PrivateKey, now time.Time) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "mypanel-" + name},
		NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: usages, DNSNames: dns, IPAddresses: ips}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		log.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		log.Fatal(err)
	}
	writePEM(filepath.Join(root, name+".crt"), "CERTIFICATE", der, 0644)
	writePEM(filepath.Join(root, name+".key"), "PRIVATE KEY", keyDER, 0600)
}

func writePEM(path, blockType string, data []byte, mode os.FileMode) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		log.Fatal(err)
	}
	if err := pem.Encode(file, &pem.Block{Type: blockType, Bytes: data}); err != nil {
		file.Close()
		log.Fatal(err)
	}
	if err := file.Close(); err != nil {
		log.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		log.Fatal(err)
	}
}

func serial() *big.Int {
	value, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatal(err)
	}
	return value
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
