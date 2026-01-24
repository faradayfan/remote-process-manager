package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/config"
)

const (
	keyPrefix    = "rpm_sk_"
	keyByteLen   = 24 // 48 hex characters
	adminKeyName = "admin"
)

type APIKeyStore struct {
	mu   sync.RWMutex
	path string
	keys *config.APIKeysConfig
}

func NewAPIKeyStore(path string) *APIKeyStore {
	return &APIKeyStore{
		path: path,
		keys: &config.APIKeysConfig{Keys: []config.APIKey{}},
	}
}

// Load reads keys from disk. Returns nil error if file doesn't exist.
func (s *APIKeyStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := config.LoadAPIKeys(s.path)
	if err != nil {
		return err
	}

	s.keys = cfg
	return nil
}

// EnsureAdminKey creates an admin key if none exists.
// Returns (key, created, error) where created=true if a new key was generated.
func (s *APIKeyStore) EnsureAdminKey() (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if admin key already exists
	for _, k := range s.keys.Keys {
		if k.Name == adminKeyName {
			return k.Key, false, nil
		}
	}

	// Generate new admin key
	key, err := generateAPIKey()
	if err != nil {
		return "", false, fmt.Errorf("generate admin key: %w", err)
	}

	adminKey := config.APIKey{
		Name:      adminKeyName,
		Key:       key,
		Roles:     []string{string(RoleAdmin)},
		CreatedAt: time.Now().UTC(),
	}

	s.keys.Keys = append(s.keys.Keys, adminKey)

	if err := config.SaveAPIKeys(s.path, s.keys); err != nil {
		// Rollback in-memory change
		s.keys.Keys = s.keys.Keys[:len(s.keys.Keys)-1]
		return "", false, fmt.Errorf("save api keys: %w", err)
	}

	return key, true, nil
}

// GetKeys returns a copy of all keys (for auth middleware initialization).
func (s *APIKeyStore) GetKeys() []config.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]config.APIKey, len(s.keys.Keys))
	copy(out, s.keys.Keys)
	return out
}

// AddKey adds a new API key. For future use (scoped tokens).
func (s *APIKeyStore) AddKey(name string, roles []string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate name
	for _, k := range s.keys.Keys {
		if k.Name == name {
			return "", fmt.Errorf("key with name %q already exists", name)
		}
	}

	key, err := generateAPIKey()
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}

	newKey := config.APIKey{
		Name:      name,
		Key:       key,
		Roles:     roles,
		CreatedAt: time.Now().UTC(),
	}

	s.keys.Keys = append(s.keys.Keys, newKey)

	if err := config.SaveAPIKeys(s.path, s.keys); err != nil {
		s.keys.Keys = s.keys.Keys[:len(s.keys.Keys)-1]
		return "", fmt.Errorf("save api keys: %w", err)
	}

	return key, nil
}

// DeleteKey removes a key by name. For future use.
func (s *APIKeyStore) DeleteKey(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, k := range s.keys.Keys {
		if k.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		return fmt.Errorf("key %q not found", name)
	}

	// Remove from slice
	removed := s.keys.Keys[idx]
	s.keys.Keys = append(s.keys.Keys[:idx], s.keys.Keys[idx+1:]...)

	if err := config.SaveAPIKeys(s.path, s.keys); err != nil {
		// Rollback
		s.keys.Keys = append(s.keys.Keys[:idx], append([]config.APIKey{removed}, s.keys.Keys[idx:]...)...)
		return fmt.Errorf("save api keys: %w", err)
	}

	return nil
}

func generateAPIKey() (string, error) {
	b := make([]byte, keyByteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return keyPrefix + hex.EncodeToString(b), nil
}

// InitAPIKeyStore loads or creates the key store and ensures admin key exists.
// Logs the admin key to stdout if newly created.
func InitAPIKeyStore(path string) (*APIKeyStore, error) {
	store := NewAPIKeyStore(path)

	if err := store.Load(); err != nil {
		return nil, fmt.Errorf("load api keys: %w", err)
	}

	key, created, err := store.EnsureAdminKey()
	if err != nil {
		return nil, fmt.Errorf("ensure admin key: %w", err)
	}

	if created {
		log.Printf("[command-server] ============================================")
		log.Printf("[command-server] ADMIN API KEY GENERATED (save this!):")
		log.Printf("[command-server]   %s", key)
		log.Printf("[command-server] ============================================")
		log.Printf("[command-server] Key saved to: %s", path)
	} else {
		log.Printf("[command-server] API keys loaded from: %s", path)
	}

	// Ensure file permissions are restrictive
	if err := os.Chmod(path, 0600); err != nil {
		log.Printf("[command-server] warning: could not set permissions on %s: %v", path, err)
	}

	return store, nil
}
