package token_test

import (
	"strings"
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/token"
)

// TestEncryptRefreshRoundTrip locks the grace-window storage contract: the raw
// replacement refresh token must be recoverable from the stored ciphertext.
func TestEncryptRefreshRoundTrip(t *testing.T) {
	service := token.NewService(strings.Repeat("a", 32), 0)

	raw := "refresh-raw-value"
	enc, err := service.EncryptRefresh(raw)
	if err != nil {
		t.Fatalf("EncryptRefresh() error = %v", err)
	}
	if len(enc) == 0 {
		t.Fatal("EncryptRefresh() returned empty ciphertext")
	}
	if strings.Contains(string(enc), raw) {
		t.Fatal("EncryptRefresh() stored the raw token in the ciphertext")
	}

	got, err := service.DecryptRefresh(enc)
	if err != nil {
		t.Fatalf("DecryptRefresh() error = %v", err)
	}
	if got != raw {
		t.Errorf("DecryptRefresh() = %q, want %q", got, raw)
	}
}

// TestEncryptRefreshUsesRandomNonce locks that two encryptions of the same token
// differ, which proves the nonce is not reused across writes.
func TestEncryptRefreshUsesRandomNonce(t *testing.T) {
	service := token.NewService(strings.Repeat("a", 32), 0)

	first, err := service.EncryptRefresh("same-value")
	if err != nil {
		t.Fatalf("EncryptRefresh() error = %v", err)
	}
	second, err := service.EncryptRefresh("same-value")
	if err != nil {
		t.Fatalf("EncryptRefresh() error = %v", err)
	}
	if string(first) == string(second) {
		t.Error("EncryptRefresh() reused the same ciphertext for equal plaintext")
	}
}

// TestDecryptRefreshRejectsTamperedCiphertext locks that AES-GCM authentication
// fails loudly instead of returning corrupt plaintext.
func TestDecryptRefreshRejectsTamperedCiphertext(t *testing.T) {
	service := token.NewService(strings.Repeat("a", 32), 0)

	enc, err := service.EncryptRefresh("refresh-raw-value")
	if err != nil {
		t.Fatalf("EncryptRefresh() error = %v", err)
	}

	tampered := append([]byte(nil), enc...)
	tampered[len(tampered)-1] ^= 0xff

	if _, err := service.DecryptRefresh(tampered); err == nil {
		t.Fatal("DecryptRefresh() error = nil, want authentication failure")
	}
}

// TestDecryptRefreshRejectsShortCiphertext locks that truncated input fails
// instead of panicking.
func TestDecryptRefreshRejectsShortCiphertext(t *testing.T) {
	service := token.NewService(strings.Repeat("a", 32), 0)

	if _, err := service.DecryptRefresh([]byte("short")); err == nil {
		t.Fatal("DecryptRefresh() error = nil, want failure for short input")
	}
	if _, err := service.DecryptRefresh(nil); err == nil {
		t.Fatal("DecryptRefresh() error = nil, want failure for empty input")
	}
}

// TestDecryptRefreshRejectsWrongKey locks that ciphertext is not readable with a
// different auth secret.
func TestDecryptRefreshRejectsWrongKey(t *testing.T) {
	encryptor := token.NewService(strings.Repeat("a", 32), 0)
	other := token.NewService(strings.Repeat("b", 32), 0)

	enc, err := encryptor.EncryptRefresh("refresh-raw-value")
	if err != nil {
		t.Fatalf("EncryptRefresh() error = %v", err)
	}

	if _, err := other.DecryptRefresh(enc); err == nil {
		t.Fatal("DecryptRefresh() error = nil, want failure with a different key")
	}
}
