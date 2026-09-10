// Package apikey implements BookDB API key generation, keyed-hash storage and
// constant-time verification.
//
// Secrets are at least 256 bits of random data. Only a keyed hash (HMAC-SHA256
// of the secret under a server-side key) plus a short non-secret lookup prefix
// are stored. The secret is shown once at creation and never persisted in
// clear. Comparison is constant-time after a bounded prefix lookup.
package apikey

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// Key is a stored API key record. The secret itself is not part of this
// struct; only the keyed hash and lookup prefix are persisted.
type Key struct {
	KeyID        string
	LookupPrefix string
	KeyHash      string
	Name         string
	Scopes       []string
	Owner        *string
	CreatedAt    time.Time
	ExpiresAt    *time.Time
	RevokedAt    *time.Time
	LastUsedAt   *time.Time
}

// IssuedKey is returned once at creation and includes the plaintext secret.
type IssuedKey struct {
	Key
	Secret string
}

// Store persists and verifies API keys.
type Store struct {
	db     *sql.DB
	macKey []byte
}

// NewStore returns a Store. macKey is the server-side key used to HMAC the
// secret; it must be at least 32 bytes.
func NewStore(db *sql.DB, macKey []byte) *Store {
	if len(macKey) < 32 {
		macKey = padMacKey(macKey)
	}
	return &Store{db: db, macKey: macKey}
}

func padMacKey(k []byte) []byte {
	out := make([]byte, 32)
	copy(out, k)
	return out
}

// GenerateSecret returns a new 256-bit random secret as a hex string.
func GenerateSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("apikey: generate secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// HashSecret computes the keyed hash of a secret.
func (s *Store) HashSecret(secret string) string {
	mac := hmac.New(sha256.New, s.macKey)
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

// LookupPrefix derives the non-secret lookup prefix from a secret. It is the
// first 8 hex chars of the keyed hash, used for bounded lookup before the
// constant-time comparison.
func (s *Store) LookupPrefix(secret string) string {
	return s.HashSecret(secret)[:8]
}

// Create generates a new key, stores its keyed hash and returns the issued
// key with the plaintext secret (shown once).
func (s *Store) Create(ctx context.Context, name string, scopes []string, owner *string, expiresAt *time.Time) (IssuedKey, error) {
	secret, err := GenerateSecret()
	if err != nil {
		return IssuedKey{}, err
	}
	hash := s.HashSecret(secret)
	prefix := hash[:8]

	var keyID string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO bookdb.api_keys (lookup_prefix, key_hash, name, scopes, owner, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING key_id`,
		prefix, hash, name, scopes, owner, expiresAt).Scan(&keyID)
	if err != nil {
		return IssuedKey{}, fmt.Errorf("apikey: create: %w", err)
	}

	return IssuedKey{
		Key: Key{
			KeyID:        keyID,
			LookupPrefix: prefix,
			KeyHash:      hash,
			Name:         name,
			Scopes:       scopes,
			Owner:        owner,
			ExpiresAt:    expiresAt,
		},
		Secret: secret,
	}, nil
}

// Verify looks up a key by its secret and returns it when valid. It returns
// ErrNotFound when the key does not exist, is revoked, or is expired.
func (s *Store) Verify(ctx context.Context, secret string) (Key, error) {
	prefix := s.LookupPrefix(secret)
	hash := s.HashSecret(secret)

	var k Key
	var owner sql.NullString
	var expiresAt, revokedAt, lastUsed sql.NullTime
	var scopes pq.StringArray
	err := s.db.QueryRowContext(ctx, `
		SELECT key_id, lookup_prefix, key_hash, name, scopes, owner, created_at, expires_at, revoked_at, last_used_at
		FROM bookdb.api_keys
		WHERE lookup_prefix = $1`, prefix).
		Scan(&k.KeyID, &k.LookupPrefix, &k.KeyHash, &k.Name, &scopes, &owner, &k.CreatedAt, &expiresAt, &revokedAt, &lastUsed)
	if err == sql.ErrNoRows {
		return Key{}, ErrNotFound
	}
	if err != nil {
		return Key{}, fmt.Errorf("apikey: verify: %w", err)
	}

	// Constant-time comparison of the keyed hash.
	if subtle.ConstantTimeCompare([]byte(k.KeyHash), []byte(hash)) != 1 {
		return Key{}, ErrNotFound
	}

	if revokedAt.Valid {
		return Key{}, ErrNotFound
	}
	if expiresAt.Valid && time.Now().UTC().After(expiresAt.Time) {
		return Key{}, ErrNotFound
	}

	if owner.Valid {
		k.Owner = &owner.String
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		k.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		k.RevokedAt = &t
	}
	if lastUsed.Valid {
		t := lastUsed.Time
		k.LastUsedAt = &t
	}
	k.Scopes = scopes
	return k, nil
}

// Revoke marks a key as revoked.
func (s *Store) Revoke(ctx context.Context, keyID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE bookdb.api_keys SET revoked_at = now() WHERE key_id = $1 AND revoked_at IS NULL`, keyID)
	if err != nil {
		return fmt.Errorf("apikey: revoke: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchLastUsed records the last-use time for a key.
func (s *Store) TouchLastUsed(ctx context.Context, keyID string) {
	_, _ = s.db.ExecContext(ctx, `UPDATE bookdb.api_keys SET last_used_at = now() WHERE key_id = $1`, keyID)
}

// ErrNotFound is returned when a key does not exist, is revoked, or is expired.
var ErrNotFound = fmt.Errorf("apikey: not found")
