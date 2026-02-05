# Crypto Package

Cryptographic utilities for API key security.

## Overview

Provides encryption and hashing functions for secure API key storage.
Uses industry-standard algorithms (AES-256-GCM, SHA-256).

## Functions

### API Key Hashing

For fast lookup without storing plaintext:

```go
hash := crypto.HashAPIKey("gw-abc123...")
// Returns: "a1b2c3d4e5f6..." (64-char hex string)
```

Used for:
- Database lookups (indexed column)
- Comparing API keys without decryption

### API Key Encryption

For secure storage with ability to retrieve/rotate:

```go
encryptor, err := crypto.NewEncryptor(os.Getenv("ENCRYPTION_KEY"))
if err != nil {
    log.Fatal(err)
}

// Encrypt
ciphertext, err := encryptor.Encrypt("gw-abc123...")
// Returns: base64-encoded ciphertext

// Decrypt
plaintext, err := encryptor.Decrypt(ciphertext)
// Returns: "gw-abc123..."
```

## Algorithms

| Function | Algorithm | Output |
|----------|-----------|--------|
| `HashAPIKey` | SHA-256 | 64-char hex string |
| `Encrypt` | AES-256-GCM | Base64 string |
| `Decrypt` | AES-256-GCM | Original plaintext |

## Key Derivation

The encryption key is derived from the `ENCRYPTION_KEY` environment variable:

```go
// Any string is accepted - it's hashed to 32 bytes
key := sha256.Sum256([]byte(envKey))
```

This means:
- Any length input works
- Same input always produces same key
- Key is never stored in plaintext

## Security Properties

### AES-256-GCM
- **Authenticated encryption**: Detects tampering
- **Random nonce**: Same plaintext produces different ciphertext
- **256-bit key**: Quantum-resistant key size

### SHA-256 Hashing
- **One-way**: Cannot recover API key from hash
- **Collision-resistant**: Practically impossible to find two keys with same hash
- **Fast**: Suitable for every request lookup

## Storage Pattern

```
┌─────────────────────────────────────────────────────┐
│                    tenants table                     │
├─────────────────────────────────────────────────────┤
│ api_key_hash      │ SHA-256 hash for lookup         │
│ api_key_encrypted │ AES-256-GCM for rotation/display│
└─────────────────────────────────────────────────────┘
```

**Lookup flow:**
1. Hash incoming API key
2. Query by `api_key_hash` (indexed)
3. Return tenant if found

**Rotation flow:**
1. Decrypt `api_key_encrypted`
2. Generate new API key
3. Update both hash and encrypted values

## Errors

| Error | Description |
|-------|-------------|
| `ErrInvalidKey` | Encryption key is not 32 bytes (after derivation) |
| `ErrInvalidCiphertext` | Ciphertext is corrupted or tampered |

## Dependencies

Only Go standard library:
- `crypto/aes`
- `crypto/cipher`
- `crypto/sha256`
- `crypto/rand`
