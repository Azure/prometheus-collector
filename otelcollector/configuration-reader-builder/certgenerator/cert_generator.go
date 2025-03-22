package certgenerator

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"github.com/prometheus-collector/certcreator"
)

const (
	KeySize = 4096
)

type CertGenerator interface {
	CreateSelfSignedCertificateKeyPair(csr *x509.Certificate) (*x509.Certificate, *rsa.PrivateKey, error)
	CreateCertificateKeyPair(csr *x509.Certificate, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error)
	CreateCertificate(csr *x509.Certificate, key *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, error)
}

type certificateGeneratorImp struct {
	certCreator certcreator.CertCreator
}

func NewCertGenerator(certCreator certcreator.CertCreator) CertGenerator {
	return &certificateGeneratorImp{
		certCreator: certCreator,
	}
}

func (c *certificateGeneratorImp) CreateSelfSignedCertificateKeyPair(csr *x509.Certificate) (*x509.Certificate, *rsa.PrivateKey, error) {
	if csr == nil {
		// return nil, nil, retry.NewError(false, fmt.Errorf("certificate signing request is nil"))
		return nil, nil, fmt.Errorf("certificate signing request is nil")
	}

	// logger := log.MustGetLogger(ctx)

	privateKey, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		fmt.Println("rsa.GenerateKeyfailed: %s", err)
		// return nil, nil, retry.NewError(true, err)
		return nil, nil, err
	}

	certificate, rerr := c.certCreator.CreateCertificateWithPublicKey(csr, &privateKey.PublicKey, csr, privateKey)
	if rerr != nil {
		fmt.Println("createCertificate failed: %+v", rerr)
		return nil, nil, rerr
	}

	return certificate, privateKey, nil
}

func (c *certificateGeneratorImp) CreateCertificateKeyPair(csr *x509.Certificate, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error) {
	if csr == nil {
		// return nil, nil, retry.NewError(false, fmt.Errorf("certificate signing request is nil"))
		return nil, nil, fmt.Errorf("certificate signing request is nil")
	}

	// logger := log.MustGetLogger(ctx)

	privateKey, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		fmt.Println("rsa.GenerateKey failed: %s", err)
		// return nil, nil, retry.NewError(true, err)
		return nil, nil, err
	}

	certificate, rerr := c.certCreator.CreateCertificateWithPublicKey(csr, &privateKey.PublicKey, caCert, caKey)
	if rerr != nil {
		fmt.Println("createCertificate failed: %+v", rerr)
		return nil, nil, rerr
	}

	return certificate, privateKey, nil
}

func (c *certificateGeneratorImp) CreateCertificate(csr *x509.Certificate, privateKey *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, error) {
	if privateKey == nil {
		// return nil, retry.NewError(false, fmt.Errorf("private key is nil"))
		return nil, fmt.Errorf("private key is nil")
	}
	if csr == nil {
		// return nil, retry.NewError(false, fmt.Errorf("certificate signing request is nil"))
		return nil, fmt.Errorf("certificate signing request is nil")
	}

	return c.certCreator.CreateCertificateWithPublicKey(csr, &privateKey.PublicKey, caCert, caKey)
}
