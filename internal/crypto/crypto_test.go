package crypto

import (
	"strings"
	"testing"
)

func TestHashAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
	}{
		{"simple key", "test-api-key"},
		{"uuid key", "gw-550e8400-e29b-41d4-a716-446655440000"},
		{"empty key", ""},
		{"special chars", "key!@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := HashAPIKey(tt.apiKey)
			hash2 := HashAPIKey(tt.apiKey)

			// Should be deterministic
			if hash1 != hash2 {
				t.Errorf("HashAPIKey not deterministic: got %s and %s", hash1, hash2)
			}

			// Should be 64 hex chars (SHA-256)
			if len(hash1) != 64 {
				t.Errorf("HashAPIKey length = %d, want 64", len(hash1))
			}

			// Should be hex encoded
			for _, c := range hash1 {
				if !strings.ContainsRune("0123456789abcdef", c) {
					t.Errorf("HashAPIKey contains non-hex char: %c", c)
				}
			}
		})
	}
}

func TestHashAPIKey_DifferentInputs(t *testing.T) {
	hash1 := HashAPIKey("key1")
	hash2 := HashAPIKey("key2")

	if hash1 == hash2 {
		t.Error("Different keys should produce different hashes")
	}
}

func TestNewEncryptor(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid key", "my-secret-key", false},
		{"empty key", "", false}, // deriveKey handles this
		{"long key", strings.Repeat("a", 100), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := NewEncryptor(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEncryptor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && enc == nil {
				t.Error("NewEncryptor() returned nil without error")
			}
		})
	}
}

func TestEncryptor_EncryptDecrypt(t *testing.T) {
	enc, err := NewEncryptor("test-encryption-key")
	if err != nil {
		t.Fatalf("NewEncryptor() error = %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple text", "hello world"},
		{"empty string", ""},
		{"json payload", `{"api_key": "sk-123", "secret": "value"}`},
		{"unicode", "こんにちは世界"},
		{"long text", strings.Repeat("a", 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Ciphertext should be different from plaintext
			if ciphertext == tt.plaintext && tt.plaintext != "" {
				t.Error("Ciphertext should not equal plaintext")
			}

			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("Decrypt() = %v, want %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptor_EncryptProducesDifferentCiphertexts(t *testing.T) {
	enc, _ := NewEncryptor("test-key")
	plaintext := "same plaintext"

	cipher1, _ := enc.Encrypt(plaintext)
	cipher2, _ := enc.Encrypt(plaintext)

	// Due to random nonce, same plaintext should produce different ciphertexts
	if cipher1 == cipher2 {
		t.Error("Encrypt should produce different ciphertexts for same plaintext (random nonce)")
	}
}

func TestEncryptor_DecryptInvalidCiphertext(t *testing.T) {
	enc, _ := NewEncryptor("test-key")

	tests := []struct {
		name       string
		ciphertext string
	}{
		{"invalid base64", "not-valid-base64!!!"},
		{"too short", "YWJj"}, // "abc" in base64, too short for nonce
		{"tampered", "dGFtcGVyZWQgZGF0YSB0aGF0IGlzIGxvbmcgZW5vdWdo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := enc.Decrypt(tt.ciphertext)
			if err == nil {
				t.Error("Decrypt() should return error for invalid ciphertext")
			}
		})
	}
}

func TestEncryptor_DifferentKeys(t *testing.T) {
	enc1, _ := NewEncryptor("key1")
	enc2, _ := NewEncryptor("key2")

	plaintext := "secret data"
	ciphertext, _ := enc1.Encrypt(plaintext)

	// Decrypting with different key should fail
	_, err := enc2.Decrypt(ciphertext)
	if err == nil {
		t.Error("Decrypt with different key should fail")
	}
}

func TestDeriveKey(t *testing.T) {
	key := deriveKey("test")

	// Should always be 32 bytes (SHA-256)
	if len(key) != 32 {
		t.Errorf("deriveKey length = %d, want 32", len(key))
	}

	// Should be deterministic
	key2 := deriveKey("test")
	for i := range key {
		if key[i] != key2[i] {
			t.Error("deriveKey not deterministic")
			break
		}
	}
}

func BenchmarkHashAPIKey(b *testing.B) {
	apiKey := "gw-550e8400-e29b-41d4-a716-446655440000"
	for i := 0; i < b.N; i++ {
		HashAPIKey(apiKey)
	}
}

func BenchmarkEncrypt(b *testing.B) {
	enc, _ := NewEncryptor("benchmark-key")
	plaintext := "benchmark plaintext data"
	for i := 0; i < b.N; i++ {
		enc.Encrypt(plaintext)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	enc, _ := NewEncryptor("benchmark-key")
	ciphertext, _ := enc.Encrypt("benchmark plaintext data")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc.Decrypt(ciphertext)
	}
}
