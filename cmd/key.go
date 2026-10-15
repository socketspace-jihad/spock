package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/muhammadjihad/spock/internal/config"
	"github.com/muhammadjihad/spock/internal/keymanager"
	"github.com/muhammadjihad/spock/internal/sshclient"
	"github.com/muhammadjihad/spock/internal/store"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage SSH keys",
	Long:  "Generate, list, show, remove, and rename SSH keys managed by Spock.",
	RunE: func(cmd *cobra.Command, args []string) error {
		keys, err := keymanager.ListKeys()
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			fmt.Println("No keys yet. Use 'spock key create --name <name>' to generate one.")
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tTYPE\tFINGERPRINT")
		for _, k := range keys {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", k.Name, k.Type, k.Fingerprint)
		}
		return tw.Flush()
	},
}

var keyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Generate a new SSH key pair",
	Example: `  spock key create --name deploy
  spock key create --name github-repo --type rsa --bits 4096
  spock key create --name secure --type ed25519 --passphrase --comment "CI deploy key"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		keyType, _ := cmd.Flags().GetString("type")
		bits, _ := cmd.Flags().GetInt("bits")
		comment, _ := cmd.Flags().GetString("comment")
		withPassphrase, _ := cmd.Flags().GetBool("passphrase")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		var passphrase string
		if withPassphrase {
			fmt.Fprint(os.Stderr, "Enter passphrase: ")
			p1, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stderr, "Confirm passphrase: ")
			p2, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}
			if string(p1) != string(p2) {
				return fmt.Errorf("passphrases do not match")
			}
			passphrase = string(p1)
		}

		path, err := keymanager.Generate(name, keyType, bits, comment, passphrase)
		if err != nil {
			return err
		}

		info, _ := keymanager.GetInfo(name)
		fmt.Println("Key created successfully!")
		fmt.Printf("  Name:        %s\n", name)
		fmt.Printf("  Type:        %s\n", keyType)
		fmt.Printf("  Path:        %s\n", path)
		if info != nil {
			fmt.Printf("  Fingerprint: %s\n", info.Fingerprint)
		}
		fmt.Printf("  Public key:  %s.pub\n", path)
		return nil
	},
}

var keyShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show key details (fingerprint, public key, metadata)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := keymanager.GetInfo(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Name:        %s\n", info.Name)
		fmt.Printf("Type:        %s\n", info.Type)
		fmt.Printf("Fingerprint: %s\n", info.Fingerprint)
		fmt.Printf("Path:        %s\n", info.Path)
		if info.Comment != "" {
			fmt.Printf("Comment:     %s\n", info.Comment)
		}
		fmt.Printf("\nPublic key:\n%s\n", info.PublicKey)
		return nil
	},
}

var keyRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Delete a key pair",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := keymanager.Delete(args[0]); err != nil {
			return err
		}
		fmt.Printf("Key %q removed.\n", args[0])
		return nil
	},
}

var keyRenameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename a key",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := keymanager.Rename(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Key renamed: %s -> %s\n", args[0], args[1])
		return nil
	},
}

var keyCopyCmd = &cobra.Command{
	Use:   "copy <name>",
	Short: "Copy the public key to clipboard",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pubKey, err := keymanager.PublicKeyString(args[0])
		if err != nil {
			return err
		}

		var clipCmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			clipCmd = exec.Command("pbcopy")
		case "linux":
			// Try xclip first, then xsel
			if _, err := exec.LookPath("xclip"); err == nil {
				clipCmd = exec.Command("xclip", "-selection", "clipboard")
			} else if _, err := exec.LookPath("xsel"); err == nil {
				clipCmd = exec.Command("xsel", "--clipboard", "--input")
			}
		}

		if clipCmd != nil {
			clipCmd.Stdin = strings.NewReader(pubKey)
			if err := clipCmd.Run(); err == nil {
				fmt.Println("Public key copied to clipboard!")
				return nil
			}
		}

		// Fallback: print the key
		fmt.Println("Could not access clipboard. Here is the public key:")
		fmt.Println(pubKey)
		return nil
	},
}

var keyPushCmd = &cobra.Command{
	Use:   "push <key-name> <connection-name>",
	Short: "Push a public key to a remote host (like ssh-copy-id)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyName := args[0]
		connName := args[1]

		pubKey, err := keymanager.PublicKeyString(keyName)
		if err != nil {
			return fmt.Errorf("key %q: %w", keyName, err)
		}

		conn, err := store.Get(connName)
		if err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		// Escape single quotes in the public key
		escaped := strings.ReplaceAll(pubKey, "'", "'\"'\"'")
		command := fmt.Sprintf(
			`mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`,
			escaped,
		)

		opts := sshclient.ConnectOptions{
			Host:    conn.Host,
			Port:    conn.Port,
			User:    conn.User,
			KeyName: conn.KeyName,
			KeyPath: conn.KeyPath,
		}

		fmt.Fprintf(os.Stderr, "Pushing key %q to %s@%s...\n", keyName, conn.User, conn.Host)
		output, err := sshclient.RunCommand(opts, command, cfg)
		if err != nil {
			if output != "" {
				fmt.Fprintln(os.Stderr, output)
			}
			return fmt.Errorf("failed to push key: %w", err)
		}
		fmt.Println("Public key installed successfully!")
		return nil
	},
}

var keyImportCmd = &cobra.Command{
	Use:   "import <path>",
	Short: "Import an existing SSH key into Spock",
	Example: `  spock key import ~/.ssh/id_rsa --name legacy
  spock key import /path/to/key --name deploy-prod`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if err := keymanager.Import(args[0], name); err != nil {
			return err
		}
		fmt.Printf("Key imported as %q.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(keyCmd)

	keyCmd.AddCommand(keyCreateCmd)
	keyCmd.AddCommand(keyShowCmd)
	keyCmd.AddCommand(keyRemoveCmd)
	keyCmd.AddCommand(keyRenameCmd)
	keyCmd.AddCommand(keyCopyCmd)
	keyCmd.AddCommand(keyPushCmd)
	keyCmd.AddCommand(keyImportCmd)

	// key create flags
	keyCreateCmd.Flags().StringP("name", "n", "", "Name for the key (required)")
	keyCreateCmd.Flags().StringP("type", "t", "ed25519", "Key type: ed25519, rsa, ecdsa")
	keyCreateCmd.Flags().IntP("bits", "b", 0, "Key size in bits (RSA: 2048/4096, ECDSA: 256/384/521)")
	keyCreateCmd.Flags().StringP("comment", "c", "", "Key comment")
	keyCreateCmd.Flags().BoolP("passphrase", "P", false, "Protect the key with a passphrase")

	// key import flags
	keyImportCmd.Flags().StringP("name", "n", "", "Name for the imported key (required)")
}
