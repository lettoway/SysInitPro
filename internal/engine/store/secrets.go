package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var secretMu sync.Mutex

func secretsFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "SysInitPro")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "secrets.enc"), nil
}

func deriveMachineKey() []byte {
	host, _ := os.Hostname()
	dir, _ := os.UserConfigDir()
	sum := sha256.Sum256([]byte("SysInitPro-v10-vault::" + host + "::" + dir))
	return sum[:]
}

func loadSecretMap() map[string]string {
	out := map[string]string{}
	p, err := secretsFilePath()
	if err != nil {
		return out
	}
	raw, err := os.ReadFile(p)
	if err != nil || len(raw) < 16 {
		return out
	}
	block, err := aes.NewCipher(deriveMachineKey())
	if err != nil {
		return out
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) < gcm.NonceSize() {
		return out
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(plain, &out)
	return out
}

func SaveSecret(key, value string) error {
	secretMu.Lock()
	defer secretMu.Unlock()

	p, err := secretsFilePath()
	if err != nil {
		return err
	}
	data := loadSecretMap()
	if value == "" {
		delete(data, key)
	} else {
		data[key] = value
	}
	plain, err := json.Marshal(data)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(deriveMachineKey())
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := gcm.Seal(nonce, nonce, plain, nil)
	return os.WriteFile(p, sealed, 0600)
}

func LoadSecret(key string) string {
	secretMu.Lock()
	defer secretMu.Unlock()
	return loadSecretMap()[key]
}
