//go:build !windows

package cloud

// Su piattaforme non-Windows non esiste DPAPI.
// encryptKey/decryptKey sono passthrough — nessuna cifratura reale.
// Vortelio è progettato principalmente per Windows.

func encryptKey(plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func decryptKey(ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}
