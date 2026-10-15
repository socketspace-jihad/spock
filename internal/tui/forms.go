package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/muhammadjihad/spock/internal/keymanager"
	"github.com/muhammadjihad/spock/internal/keyring"
	"github.com/muhammadjihad/spock/internal/store"
)

type FormAction int

const (
	FormNone FormAction = iota
	FormAddConn
	FormEditConn
	FormCreateKey
	FormSelectUser
)

type FormResult struct {
	action FormAction
	err    error
}

type formValues struct {
	name         string
	host         string
	user         string
	port         string
	key          string
	keyType      string
	keyBits      string
	comment      string
	jumpHost     string
	password     string
	selectedUser string // for user selection dropdown
	connName     string // connection name for user selection context
}

func newConnForm(vals *formValues) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name").
				Description("Friendly name for this connection").
				Value(&vals.name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Host").
				Description("Hostname or IP address").
				Value(&vals.host).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("host is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("User").
				Description("SSH username").
				Value(&vals.user),
			huh.NewInput().
				Title("Port").
				Description("SSH port (default: 22)").
				Value(&vals.port),
			huh.NewInput().
				Title("Key").
				Description("Spock-managed key name (optional)").
				Value(&vals.key),
			huh.NewInput().
				Title("Jump Host").
				Description("ProxyJump host (optional)").
				Value(&vals.jumpHost),
			huh.NewInput().
				Title("Password").
				Description("Saved in OS keychain (optional)").
				EchoMode(huh.EchoModePassword).
				Value(&vals.password),
		),
	).WithTheme(huh.ThemeCatppuccin())
}

func newKeyForm(vals *formValues) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name").
				Description("Friendly name for this key").
				Value(&vals.name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Type").
				Description("Key algorithm").
				Options(
					huh.NewOption("Ed25519 (recommended)", "ed25519"),
					huh.NewOption("RSA (4096-bit)", "rsa"),
					huh.NewOption("ECDSA (256-bit)", "ecdsa"),
				).
				Value(&vals.keyType),
			huh.NewInput().
				Title("Comment").
				Description("Key comment (optional)").
				Value(&vals.comment),
		),
	).WithTheme(huh.ThemeCatppuccin())
}

func submitConnForm(vals *formValues, editing bool) error {
	port := 22
	if vals.port != "" {
		fmt.Sscanf(vals.port, "%d", &port)
	}
	user := vals.user
	if user == "" {
		user = "root"
	}

	conn := &store.Connection{
		Name:        vals.name,
		Host:        vals.host,
		User:        user,
		Port:        port,
		KeyName:     vals.key,
		JumpHost:    vals.jumpHost,
		HasPassword: vals.password != "",
	}

	if vals.password != "" {
		if err := keyring.SetPassword(vals.name, vals.password); err != nil {
			return fmt.Errorf("failed to save password to keychain: %w", err)
		}
	}

	if editing {
		return store.Update(conn)
	}
	return store.Add(conn)
}

func submitKeyForm(vals *formValues) error {
	bits := 0
	if vals.keyType == "rsa" {
		bits = 4096
	}
	_, err := keymanager.Generate(vals.name, vals.keyType, bits, vals.comment, "")
	return err
}

// newUserSelectForm builds a dropdown to pick which user to SSH as.
func newUserSelectForm(conn *store.Connection, vals *formValues) *huh.Form {
	// Build unique user list: primary user first, then extras
	seen := map[string]bool{}
	var options []huh.Option[string]

	addUser := func(u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			options = append(options, huh.NewOption(u, u))
		}
	}

	addUser(conn.User)
	for _, u := range conn.Users {
		addUser(u)
	}
	options = append(options, huh.NewOption("+ Enter custom user...", "__custom__"))

	vals.selectedUser = conn.User

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Connect as").
				Description(fmt.Sprintf("Select user for %s", conn.Name)).
				Options(options...).
				Value(&vals.selectedUser),
		),
	).WithTheme(huh.ThemeCatppuccin())
}

// newCustomUserForm shows a text input for entering a custom username.
func newCustomUserForm(vals *formValues) *huh.Form {
	vals.selectedUser = ""
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Username").
				Description("Enter the SSH username").
				Value(&vals.selectedUser).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("username is required")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeCatppuccin())
}
