package certoperator

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/prometheus-collector/certgenerator"
)

type certOperatorImp struct {
	certGenerator certgenerator.CertGenerator
}

type CertOperator interface {
	certificateToPem(cert *x509.Certificate) ([]byte, error)
	privateKeyToPem(privateKey *rsa.PrivateKey) ([]byte, error)
	pemToCertificate(raw string) (*x509.Certificate, error)
	pemToPrivateKey(raw string) (*rsa.PrivateKey, error)
	CreateCertificateKeyPair(csr *x509.Certificate,
		caCert *x509.Certificate,
		caKey *rsa.PrivateKey) (string, string, error)
	CreateSelfSignedCertificateKeyPair(
		csr *x509.Certificate) (*x509.Certificate, string, *rsa.PrivateKey, string, error)
}

func NewCertOperator(certGenerator certgenerator.CertGenerator) CertOperator {
	return &certOperatorImp{certGenerator: certGenerator}
}

var encodeFunc = pem.Encode

func (o *certOperatorImp) pemToPrivateKey(raw string) (*rsa.PrivateKey, error) {
	kpb, _ := pem.Decode([]byte(raw))
	if kpb == nil {
		fmt.Println("Decode returns nil")
		return nil, errors.New("The raw pem is not a valid PEM formatted block")
	}
	return x509.ParsePKCS1PrivateKey(kpb.Bytes)
}

func (o *certOperatorImp) CreateSelfSignedCertificateKeyPair(
	csr *x509.Certificate) (*x509.Certificate, string, *rsa.PrivateKey, string, error) {

	cert, key, rerr := o.certGenerator.CreateSelfSignedCertificateKeyPair(csr)
	if rerr != nil {
		fmt.Println("CreateSelfSignedCertificateKeyPair failed: %v", rerr)
		return nil, "", nil, "", rerr
	}
	certPem, keyPem, err := o.getCertKeyAsPem(cert, key)
	if err != nil {
		fmt.Println("certKeyToPem failed: %s", err)
		return nil, "", nil, "", err
	}
	fmt.Println("self signed certificate %v is generated successfully", csr.Subject.CommonName)
	return cert, certPem, key, keyPem, nil
}

func (o *certOperatorImp) pemToCertificate(raw string) (*x509.Certificate, error) {
	cpb, _ := pem.Decode([]byte(raw))
	if cpb == nil {
		fmt.Println("Decode returns nil")
		return nil, errors.New("The raw pem is not a valid PEM formatted block")
	}
	return x509.ParseCertificate(cpb.Bytes)
}

func (o *certOperatorImp) certificateToPem(cert *x509.Certificate) ([]byte, error) {
	derBytes := cert.Raw
	pemBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	}
	pemBuffer := bytes.Buffer{}
	err := encodeFunc(&pemBuffer, pemBlock)
	if err != nil {
		fmt.Println("pem encode() return error %s", err)
		return nil, err
	}

	return pemBuffer.Bytes(), nil
}

func (o *certOperatorImp) privateKeyToPem(privateKey *rsa.PrivateKey) ([]byte, error) {
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	pemBuffer := bytes.Buffer{}
	err := encodeFunc(&pemBuffer, pemBlock)
	if err != nil {
		fmt.Println("pem encode() return error %s", err)
		return nil, err
	}

	return pemBuffer.Bytes(), nil
}

func (o *certOperatorImp) CreateCertificateKeyPair(
	csr *x509.Certificate,
	caCert *x509.Certificate,
	caKey *rsa.PrivateKey) (string, string, error) {
	cert, key, rerr := o.certGenerator.CreateCertificateKeyPair(csr, caCert, caKey)
	if rerr != nil {
		fmt.Println("CreateCertificateKeyPair failed: %v", rerr)
		return "", "", rerr
	}
	certPem, keyPem, err := o.getCertKeyAsPem(cert, key)
	if err != nil {
		fmt.Println("getCertKeyAsPem failed: %s", err)
		return "", "", err
	}
	fmt.Println("certificate %v is generated successfully", csr.Subject.CommonName)
	return certPem, keyPem, nil
}

func (o *certOperatorImp) CreateCertificate(
	csr *x509.Certificate,
	keyPem string,
	caCert *x509.Certificate,
	caKey *rsa.PrivateKey) (string, error) {
	key, err := o.pemToPrivateKey(keyPem)
	if err != nil {
		fmt.Println("PemToPrivateKey failed: %s", err)
		return "", err
	}

	cert, rerr := o.certGenerator.CreateCertificate(csr, key, caCert, caKey)
	if rerr != nil {
		fmt.Println("CreateCertificate failed: %v", rerr)
		return "", rerr
	}

	certBytes, err := o.certificateToPem(cert)
	if err != nil {
		fmt.Println("CertificateToPem failed: %s", err)
		return "", err
	}
	fmt.Println("certificate %v is generated successfully", csr.Subject.CommonName)
	return string(certBytes), nil
}

func (o *certOperatorImp) getCertKeyAsPem(
	cert *x509.Certificate,
	key *rsa.PrivateKey) (string, string, error) {
	certBytes, err := o.certificateToPem(cert)
	if err != nil {
		fmt.Println("CertificateToPem failed: %s", err)
		return "", "", err
	}

	keyBytes, err := o.privateKeyToPem(key)
	if err != nil {
		fmt.Println("PrivateKeyToPem failed: %s", err)
		return "", "", err
	}

	return string(certBytes), string(keyBytes), nil
}
