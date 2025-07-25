package shared

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
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

		// Handle UTF-8 BOM
		if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
			fmt.Println("  Found UTF-8 BOM, stripping.")
			data = data[3:]
		}

		// Handle UTF-16 LE BOM
		if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) || bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
			fmt.Println("  Found UTF-16 BOM, decoding.")
			utf16Decoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
			decoded, _, err := transform.Bytes(utf16Decoder, data)
			if err != nil {
				fmt.Printf("  UTF-16 decode failed: %v\n", err)
				continue
			}
			data = decoded
		}

		// Trim leading whitespace/newlines just in case
		data = bytes.TrimLeft(data, "\r\n\t ")

		block, _ := pem.Decode(data)
		if block == nil {
			fmt.Printf("  Skipping: PEM decode returned nil.\n")
			continue
		}

		if !strings.EqualFold(block.Type, "CERTIFICATE") {
			fmt.Printf("  Skipping: block type is %q, not CERTIFICATE.\n", block.Type)
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
