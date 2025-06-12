package shared

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func BootstrapCACertificates() {
	certMountPath := `C:\ca`

	files, err := os.ReadDir(certMountPath)
	if err != nil {
		fmt.Printf("Unable to read certificate directory %s: %v\n", certMountPath, err)
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		absolutePath := filepath.Join(certMountPath, file.Name())
		fmt.Printf("Processing certificate: %s\n", absolutePath)

		data, err := os.ReadFile(absolutePath)
		if err != nil {
			fmt.Printf("  Failed to read file: %v\n", err)
			continue
		}

		block, _ := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" {
			fmt.Printf("  Skipping invalid or non-certificate PEM block.\n")
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			fmt.Printf("  Certificate parse error: %v\n", err)
			continue
		}

		if err := addCertToRootStore(cert.Raw); err != nil {
			fmt.Printf("  Import failed: %v\n", err)
		} else {
			fmt.Printf("  Certificate imported successfully.\n")
		}
	}
}

func addCertToRootStore(certDER []byte) error {
	const ROOT_STORE = `ROOT`

	storeNamePtr, err := windows.UTF16PtrFromString(ROOT_STORE)
	if err != nil {
		return fmt.Errorf("failed to encode store name: %w", err)
	}

	store, err := windows.CertOpenStore(
		windows.CERT_STORE_PROV_SYSTEM,
		0,
		0,
		windows.CERT_SYSTEM_STORE_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(storeNamePtr)), // ✅ cast to uintptr
	)
	if err != nil {
		return fmt.Errorf("opening root store: %w", err)
	}
	defer windows.CertCloseStore(store, 0)

	certContext, err := windows.CertCreateCertificateContext(
		windows.X509_ASN_ENCODING,
		&certDER[0],
		uint32(len(certDER)),
	)
	if err != nil {
		return fmt.Errorf("creating certificate context: %w", err)
	}
	defer windows.CertFreeCertificateContext(certContext)

	err = windows.CertAddCertificateContextToStore(
		store,
		certContext,
		windows.CERT_STORE_ADD_REPLACE_EXISTING,
		nil,
	)
	if err != nil {
		return fmt.Errorf("adding certificate to store failed: %w", err)
	}

	return nil
}
