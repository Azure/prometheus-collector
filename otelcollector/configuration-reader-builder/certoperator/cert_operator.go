package certoperator

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/Azure/webhook-tls-manager/toolkit/certificates/certgenerator"
	"github.com/Azure/webhook-tls-manager/toolkit/log"
	"k8s.io/legacy-cloud-providers/azure/retry"
)

type certOperatorImp struct {
	certGenerator certgenerator.CertGenerator
}

func NewCertOperator(certGenerator certgenerator.CertGenerator) CertOperator {
	return &certOperatorImp{certGenerator: certGenerator}
}

var encodeFunc = pem.Encode

func (o *certOperatorImp) pemToPrivateKey(ctx context.Context, raw string) (*rsa.PrivateKey, error) {
	kpb, _ := pem.Decode([]byte(raw))
	if kpb == nil {
		log.MustGetLogger(ctx).Errorf("Decode returns nil")
		return nil, errors.New("The raw pem is not a valid PEM formatted block")
	}
	return x509.ParsePKCS1PrivateKey(kpb.Bytes)
}

func (o *certOperatorImp) CreateSelfSignedCertificateKeyPair(
	ctx context.Context,
	csr *x509.Certificate) (*x509.Certificate, string, *rsa.PrivateKey, string, *retry.Error) {

	cert, key, rerr := o.certGenerator.CreateSelfSignedCertificateKeyPair(ctx, csr)
	if rerr != nil {
		log.MustGetLogger(ctx).Errorf("CreateSelfSignedCertificateKeyPair failed: %v", rerr)
		return nil, "", nil, "", rerr
	}
	certPem, keyPem, err := o.getCertKeyAsPem(ctx, cert, key)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("certKeyToPem failed: %s", err)
		return nil, "", nil, "", retry.NewError(false, err)
	}
	log.MustGetLogger(ctx).Infof("self signed certificate '%v' is generated successfully", csr.Subject.CommonName)
	return cert, certPem, key, keyPem, nil
}

func (o *certOperatorImp) pemToCertificate(ctx context.Context, raw string) (*x509.Certificate, error) {
	cpb, _ := pem.Decode([]byte(raw))
	if cpb == nil {
		log.MustGetLogger(ctx).Errorf("Decode returns nil")
		return nil, errors.New("The raw pem is not a valid PEM formatted block")
	}
	return x509.ParseCertificate(cpb.Bytes)
}

func (o *certOperatorImp) certificateToPem(ctx context.Context, cert *x509.Certificate) ([]byte, error) {
	derBytes := cert.Raw
	pemBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	}
	pemBuffer := bytes.Buffer{}
	err := encodeFunc(&pemBuffer, pemBlock)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("pem encode() return error %s", err)
		return nil, err
	}

	return pemBuffer.Bytes(), nil
}

func (o *certOperatorImp) privateKeyToPem(ctx context.Context, privateKey *rsa.PrivateKey) ([]byte, error) {
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	pemBuffer := bytes.Buffer{}
	err := encodeFunc(&pemBuffer, pemBlock)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("pem encode() return error %s", err)
		return nil, err
	}

	return pemBuffer.Bytes(), nil
}

func (o *certOperatorImp) CreateCertificateKeyPair(
	ctx context.Context,
	csr *x509.Certificate,
	caCert *x509.Certificate,
	caKey *rsa.PrivateKey) (string, string, *retry.Error) {
	cert, key, rerr := o.certGenerator.CreateCertificateKeyPair(ctx, csr, caCert, caKey)
	if rerr != nil {
		log.MustGetLogger(ctx).Errorf("CreateCertificateKeyPair failed: %v", rerr)
		return "", "", rerr
	}
	certPem, keyPem, err := o.getCertKeyAsPem(ctx, cert, key)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("getCertKeyAsPem failed: %s", err)
		return "", "", retry.NewError(false, err)
	}
	log.MustGetLogger(ctx).Infof("certificate %v is generated successfully", csr.Subject.CommonName)
	return certPem, keyPem, nil
}

func (o *certOperatorImp) CreateCertificate(
	ctx context.Context,
	csr *x509.Certificate,
	keyPem string,
	caCert *x509.Certificate,
	caKey *rsa.PrivateKey) (string, *retry.Error) {
	key, err := o.pemToPrivateKey(ctx, keyPem)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("PemToPrivateKey failed: %s", err)
		return "", retry.NewError(false, err)
	}

	cert, rerr := o.certGenerator.CreateCertificate(ctx, csr, key, caCert, caKey)
	if rerr != nil {
		log.MustGetLogger(ctx).Errorf("CreateCertificate failed: %v", rerr)
		return "", rerr
	}

	certBytes, err := o.certificateToPem(ctx, cert)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("CertificateToPem failed: %s", err)
		return "", retry.NewError(false, err)
	}
	log.MustGetLogger(ctx).Infof("certificate %v is generated successfully", csr.Subject.CommonName)
	return string(certBytes), nil
}

func (o *certOperatorImp) getCertKeyAsPem(
	ctx context.Context,
	cert *x509.Certificate,
	key *rsa.PrivateKey) (string, string, error) {
	certBytes, err := o.certificateToPem(ctx, cert)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("CertificateToPem failed: %s", err)
		return "", "", err
	}

	keyBytes, err := o.privateKeyToPem(ctx, key)
	if err != nil {
		log.MustGetLogger(ctx).Errorf("PrivateKeyToPem failed: %s", err)
		return "", "", err
	}

	return string(certBytes), string(keyBytes), nil
}
