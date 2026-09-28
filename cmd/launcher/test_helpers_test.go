package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func releaseRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func readRepoText(t *testing.T, relative string) string {
	t.Helper()
	root := releaseRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return string(data)
}

type memoryProviderCredentialStore struct {
	mu      sync.Mutex
	secrets map[string]string
}

func newMemoryProviderCredentialStore() *memoryProviderCredentialStore {
	return &memoryProviderCredentialStore{secrets: map[string]string{}}
}

func (s *memoryProviderCredentialStore) Backend() string { return "memory-test" }

func (s *memoryProviderCredentialStore) Put(providerID, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[providerID] = secret
	return nil
}

func (s *memoryProviderCredentialStore) Get(providerID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.secrets[providerID]
	if !ok {
		return "", errCredentialNotFound
	}
	return value, nil
}

func (s *memoryProviderCredentialStore) Delete(providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, providerID)
	return nil
}
