package security

// ConnectionDSN returns a driver-ready DSN from stored metadata.
// When enc is non-nil and the stored value matches the encrypted heuristic, it is decrypted;
// otherwise stored is returned unchanged (plaintext when encryption is off or legacy rows).
func ConnectionDSN(enc *Encryption, stored string) (string, error) {
	if enc != nil && enc.IsEncrypted(stored) {
		return enc.Decrypt(stored)
	}
	return stored, nil
}
