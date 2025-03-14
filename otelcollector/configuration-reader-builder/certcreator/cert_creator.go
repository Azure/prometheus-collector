package certcreator

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"io"
	"math/big"

	"github.com/Azure/webhook-tls-manager/toolkit/log"
	"k8s.io/legacy-cloud-providers/azure/retry"
)

type CertCreatorImp struct {
}

func NewCertCreator() CertCreator {
	return &CertCreatorImp{}
}

func (c *CertCreatorImp) GenerateSN() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func (c *CertCreatorImp) CreateCertificate(rand io.Reader, template, parent *x509.Certificate, publicKey interface{}, privateKey interface{}) ([]byte, error) {
	return x509.CreateCertificate(rand, template, parent, publicKey, privateKey)
}

func (c *CertCreatorImp) ParseCertificate(derBytes []byte) (*x509.Certificate, error) {
	return x509.ParseCertificate(derBytes)
}

func (c *CertCreatorImp) CreateCertificateWithPublicKey(ctx context.Context, csr *x509.Certificate, publicKey *rsa.PublicKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, *retry.Error) {
	sn, err := c.GenerateSN()
	if err != nil {
		log.MustGetLogger(ctx).Errorf("generate serial number failed: %s", err)
		return nil, retry.NewError(false, err)
	}
	csr.SerialNumber = sn

	certDerBytes, err := c.CreateCertificate(rand.Reader, csr, caCert, publicKey, caKey)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("createCertificateFunc failed: %s", err)
		return nil, retry.NewError(false, err)
	}

	certificate, err := c.ParseCertificate(certDerBytes)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("parseCertificateFunc failed: %s", err)
		return nil, retry.NewError(false, err)
	}

	log.MustGetLogger(ctx).Info("certificate created successfully")
	return certificate, nil
}
