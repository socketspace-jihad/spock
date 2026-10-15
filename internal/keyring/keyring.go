package keyring

import (
	"fmt"

	zkr "github.com/zalando/go-keyring"
)

const serviceName = "spock-ssh"

// SetPassword stores a connection's password in the OS keychain.
func SetPassword(connectionName, password string) error {
	if err := zkr.Set(serviceName, connectionName, password); err != nil {
		return fmt.Errorf("keychain: %w", err)
	}
	return nil
}

// GetPassword retrieves a connection's password from the OS keychain.
// Returns empty string and nil error if no password is stored.
func GetPassword(connectionName string) (string, error) {
	pw, err := zkr.Get(serviceName, connectionName)
	if err == zkr.ErrNotFound {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("keychain: %w", err)
	}
	return pw, nil
}

// DeletePassword removes a connection's password from the OS keychain.
func DeletePassword(connectionName string) error {
	err := zkr.Delete(serviceName, connectionName)
	if err == zkr.ErrNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("keychain: %w", err)
	}
	return nil
}
