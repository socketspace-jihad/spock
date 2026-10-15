package keymanager

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/muhammadjihad/spock/internal/config"
)

type KeyInfo struct {
	Name        string
	Path        string
	Type        string
	Fingerprint string
	PublicKey   string
	Comment     string
}

func Generate(name, keyType string, bits int, comment, passphrase string) (string, error) {
	if err := config.EnsureDirs(); err != nil {
		return "", err
	}

	keyPath := filepath.Join(config.KeysDir(), name)
	if _, err := os.Stat(keyPath); err == nil {
		return "", fmt.Errorf("key %q already exists", name)
	}

	var privateKey any
	var err error

	switch keyType {
	case "rsa":
		if bits == 0 {
			bits = 4096
		}
		privateKey, err = rsa.GenerateKey(rand.Reader, bits)
	case "ed25519":
		_, privateKey, err = ed25519.GenerateKey(rand.Reader)
	case "ecdsa":
		if bits == 0 {
			bits = 256
		}
		var curve elliptic.Curve
		switch bits {
		case 256:
			curve = elliptic.P256()
		case 384:
			curve = elliptic.P384()
		case 521:
			curve = elliptic.P521()
		default:
			return "", fmt.Errorf("unsupported ECDSA curve: %d (use 256, 384, or 521)", bits)
		}
		privateKey, err = ecdsa.GenerateKey(curve, rand.Reader)
	default:
		return "", fmt.Errorf("unsupported key type: %s (use ed25519, rsa, or ecdsa)", keyType)
	}
	if err != nil {
		return "", fmt.Errorf("key generation failed: %w", err)
	}

	if comment == "" {
		hostname, _ := os.Hostname()
		comment = fmt.Sprintf("spock-%s@%s", name, hostname)
	}

	// Marshal private key in OpenSSH format
	var block *pem.Block
	if passphrase != "" {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(privateKey, comment, []byte(passphrase))
	} else {
		block, err = ssh.MarshalPrivateKey(privateKey, comment)
	}
	if err != nil {
		return "", fmt.Errorf("failed to marshal private key: %w", err)
	}

	privFile, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer privFile.Close()
	if err := pem.Encode(privFile, block); err != nil {
		return "", err
	}

	// Write public key
	pubKey, err := ssh.NewPublicKey(publicKey(privateKey))
	if err != nil {
		return "", fmt.Errorf("failed to derive public key: %w", err)
	}
	pubBytes := ssh.MarshalAuthorizedKey(pubKey)
	// Append comment
	pubLine := strings.TrimSpace(string(pubBytes)) + " " + comment + "\n"
	if err := os.WriteFile(keyPath+".pub", []byte(pubLine), 0644); err != nil {
		return "", err
	}

	return keyPath, nil
}

func GetInfo(name string) (*KeyInfo, error) {
	keyPath := filepath.Join(config.KeysDir(), name)
	pubPath := keyPath + ".pub"

	pubData, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, fmt.Errorf("public key not found for %q", name)
	}

	pubKey, comment, _, _, err := ssh.ParseAuthorizedKey(pubData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return &KeyInfo{
		Name:        name,
		Path:        keyPath,
		Type:        pubKey.Type(),
		Fingerprint: ssh.FingerprintSHA256(pubKey),
		PublicKey:   strings.TrimSpace(string(pubData)),
		Comment:     comment,
	}, nil
}

func ListKeys() ([]KeyInfo, error) {
	entries, err := os.ReadDir(config.KeysDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var keys []KeyInfo
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}
		info, err := GetInfo(entry.Name())
		if err != nil {
			continue
		}
		keys = append(keys, *info)
	}
	return keys, nil
}

func Delete(name string) error {
	keyPath := filepath.Join(config.KeysDir(), name)
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("key %q not found", name)
	}
	os.Remove(keyPath + ".pub")
	return os.Remove(keyPath)
}

func Rename(oldName, newName string) error {
	oldPath := filepath.Join(config.KeysDir(), oldName)
	newPath := filepath.Join(config.KeysDir(), newName)

	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return fmt.Errorf("key %q not found", oldName)
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("key %q already exists", newName)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	oldPub := oldPath + ".pub"
	if _, err := os.Stat(oldPub); err == nil {
		return os.Rename(oldPub, newPath+".pub")
	}
	return nil
}

func Import(srcPath, name string) error {
	if err := config.EnsureDirs(); err != nil {
		return err
	}
	dstPath := filepath.Join(config.KeysDir(), name)
	if _, err := os.Stat(dstPath); err == nil {
		return fmt.Errorf("key %q already exists", name)
	}

	// Copy private key
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read key: %w", err)
	}
	if err := os.WriteFile(dstPath, data, 0600); err != nil {
		return err
	}

	// Copy public key if exists
	pubSrc := srcPath + ".pub"
	if pubData, err := os.ReadFile(pubSrc); err == nil {
		os.WriteFile(dstPath+".pub", pubData, 0644)
	}

	return nil
}

func PublicKeyString(name string) (string, error) {
	info, err := GetInfo(name)
	if err != nil {
		return "", err
	}
	return info.PublicKey, nil
}

func publicKey(priv any) any {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public()
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}
