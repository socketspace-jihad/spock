package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/muhammadjihad/spock/internal/config"
	"github.com/muhammadjihad/spock/internal/keyring"
	"github.com/muhammadjihad/spock/internal/session"
	"github.com/muhammadjihad/spock/internal/sshclient"
	"github.com/muhammadjihad/spock/internal/store"
)

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "Manage SSH connections",
	Long:  "List, add, edit, remove, and connect to SSH hosts.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default: list connections
		conns, err := store.List()
		if err != nil {
			return err
		}
		if len(conns) == 0 {
			fmt.Println("No connections saved yet. Use 'spock ssh add <name>' to add one.")
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tHOST\tUSER\tPORT\tKEY\tLAST CONNECTED")
		for _, c := range conns {
			key := c.KeyName
			if key == "" && c.KeyPath != "" {
				key = filepath.Base(c.KeyPath)
			}
			if key == "" {
				key = "-"
			}
			last := "-"
			if c.LastConnected != nil {
				last = c.LastConnected.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n", c.Name, c.Host, c.User, c.Port, key, last)
		}
		return tw.Flush()
	},
}

var sshToCmd = &cobra.Command{
	Use:     "to <name>",
	Short:   "Connect to a saved SSH host",
	Example: "  spock ssh to worker-1",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		conn, err := store.Get(name)
		if err != nil {
			return err
		}

		// Override user if flag is set
		if u, _ := cmd.Flags().GetString("user"); u != "" {
			conn.User = u
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		// Register session
		sess, err := session.Register(name, conn.Host, conn.User)
		if err != nil {
			return fmt.Errorf("session registration failed: %w", err)
		}
		defer session.Deregister(sess.ID)

		store.Touch(name)

		// Set up logging
		var logWriter *os.File
		if cfg.SessionLogging {
			logWriter, err = os.Create(sess.LogFile)
			if err != nil {
				logWriter = nil
			} else {
				defer logWriter.Close()
			}
		}

		broadcaster := session.NewBroadcaster(os.Stdout, logWriter)
		defer broadcaster.Close()

		// Start mirror server
		mirrorSrv, err := session.StartMirrorServer(sess.SocketPath, broadcaster)
		if err == nil {
			defer mirrorSrv.Close()
		}

		// Retrieve stored password from OS keychain
		password, _ := keyring.GetPassword(name)

		opts := sshclient.ConnectOptions{
			Host:     conn.Host,
			Port:     conn.Port,
			User:     conn.User,
			KeyName:  conn.KeyName,
			KeyPath:  conn.KeyPath,
			Password: password,
		}

		// Auto-describe node on first connect (gather OS, CPU, memory info)
		if conn.NodeInfo == nil {
			fmt.Fprintf(os.Stderr, "First connect — gathering node info...\n")
			if info, derr := sshclient.DescribeNode(opts, cfg); derr == nil {
				saveNodeInfo(name, info)
			}
		}

		fmt.Fprintf(os.Stderr, "Connecting to %s@%s:%d (session: %s)...\n", conn.User, conn.Host, conn.Port, sess.ID)
		return sshclient.Connect(opts, broadcaster, cfg)
	},
}

var sshAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new SSH connection",
	Example: `  spock ssh add worker-1 --host 10.0.0.5 --user root
  spock ssh add staging --host staging.example.com --user deploy --key deploy-key --port 2222
  spock ssh add prod --host prod.example.com --user admin --password`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		host, _ := cmd.Flags().GetString("host")
		user, _ := cmd.Flags().GetString("user")
		port, _ := cmd.Flags().GetInt("port")
		keyName, _ := cmd.Flags().GetString("key")
		keyPath, _ := cmd.Flags().GetString("key-path")
		jumpHost, _ := cmd.Flags().GetString("jump")
		withPassword, _ := cmd.Flags().GetBool("password")

		if host == "" {
			return fmt.Errorf("--host is required")
		}
		if user == "" {
			user = os.Getenv("USER")
		}

		conn := &store.Connection{
			Name:     name,
			Host:     host,
			Port:     port,
			User:     user,
			KeyName:  keyName,
			KeyPath:  keyPath,
			JumpHost: jumpHost,
		}

		if withPassword {
			fmt.Fprint(os.Stderr, "Password: ")
			pw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}
			if err := keyring.SetPassword(name, string(pw)); err != nil {
				return fmt.Errorf("failed to save password to keychain: %w", err)
			}
			conn.HasPassword = true
		}

		if err := store.Add(conn); err != nil {
			return err
		}
		fmt.Printf("Connection %q added.\n", name)
		return nil
	},
}

var sshEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit an existing SSH connection",
	Example: `  spock ssh edit worker-1 --port 2222
  spock ssh edit staging --user admin --key new-key
  spock ssh edit prod --password
  spock ssh edit prod --clear-password`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		conn, err := store.Get(name)
		if err != nil {
			return err
		}

		if cmd.Flags().Changed("host") {
			conn.Host, _ = cmd.Flags().GetString("host")
		}
		if cmd.Flags().Changed("user") {
			conn.User, _ = cmd.Flags().GetString("user")
		}
		if cmd.Flags().Changed("port") {
			conn.Port, _ = cmd.Flags().GetInt("port")
		}
		if cmd.Flags().Changed("key") {
			conn.KeyName, _ = cmd.Flags().GetString("key")
		}
		if cmd.Flags().Changed("key-path") {
			conn.KeyPath, _ = cmd.Flags().GetString("key-path")
		}
		if cmd.Flags().Changed("jump") {
			conn.JumpHost, _ = cmd.Flags().GetString("jump")
		}

		if cmd.Flags().Changed("password") {
			fmt.Fprint(os.Stderr, "New password: ")
			pw, perr := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if perr != nil {
				return perr
			}
			if err := keyring.SetPassword(name, string(pw)); err != nil {
				return fmt.Errorf("failed to save password to keychain: %w", err)
			}
			conn.HasPassword = true
		}

		if cmd.Flags().Changed("clear-password") {
			keyring.DeletePassword(name)
			conn.HasPassword = false
		}

		if err := store.Update(conn); err != nil {
			return err
		}
		fmt.Printf("Connection %q updated.\n", name)
		return nil
	},
}

var sshRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove a saved SSH connection",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyring.DeletePassword(args[0])
		if err := store.Remove(args[0]); err != nil {
			return err
		}
		fmt.Printf("Connection %q removed.\n", args[0])
		return nil
	},
}

var sshShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a saved connection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := store.Get(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Name:      %s\n", conn.Name)
		fmt.Printf("Host:      %s\n", conn.Host)
		fmt.Printf("Port:      %d\n", conn.Port)
		fmt.Printf("User:      %s\n", conn.User)
		if conn.KeyName != "" {
			fmt.Printf("Key:       %s (managed)\n", conn.KeyName)
		}
		if conn.KeyPath != "" {
			fmt.Printf("Key path:  %s\n", conn.KeyPath)
		}
		if conn.JumpHost != "" {
			fmt.Printf("Jump host: %s\n", conn.JumpHost)
		}
		if conn.HasPassword {
			fmt.Printf("Password:  saved in keychain\n")
		}
		fmt.Printf("Created:   %s\n", conn.CreatedAt.Format("2006-01-02 15:04"))
		if conn.LastConnected != nil {
			fmt.Printf("Last used: %s\n", conn.LastConnected.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var sshTestCmd = &cobra.Command{
	Use:     "test <name>",
	Short:   "Test connectivity to a saved SSH host",
	Example: "  spock ssh test worker-1",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := store.Get(args[0])
		if err != nil {
			return err
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Testing %s@%s:%d... ", conn.User, conn.Host, conn.Port)

		opts := sshclient.ConnectOptions{
			Host:    conn.Host,
			Port:    conn.Port,
			User:    conn.User,
			KeyName: conn.KeyName,
			KeyPath: conn.KeyPath,
		}
		elapsed, err := sshclient.TestConnection(opts, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL (%s)\n", elapsed.Round(1e6))
			return fmt.Errorf("connection failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "OK (%s)\n", elapsed.Round(1e6))
		return nil
	},
}

var sshCopyCmd = &cobra.Command{
	Use:   "copy <source> <destination>",
	Short: "Duplicate a connection profile under a new name",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := store.Copy(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Connection %q copied to %q.\n", args[0], args[1])
		return nil
	},
}

var sshImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import connections from ~/.ssh/config",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		sshConfigPath := filepath.Join(home, ".ssh", "config")

		conns, err := parseSSHConfig(sshConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read SSH config: %w", err)
		}
		if len(conns) == 0 {
			fmt.Println("No hosts found in SSH config.")
			return nil
		}

		added, skipped, err := store.ImportBatch(conns)
		if err != nil {
			return err
		}
		fmt.Printf("Imported %d connection(s), skipped %d (already exist).\n", added, skipped)
		return nil
	},
}

var sshExportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Print a connection as an SSH config block",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := store.Get(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Host %s\n", conn.Name)
		fmt.Printf("    HostName %s\n", conn.Host)
		fmt.Printf("    User %s\n", conn.User)
		if conn.Port != 22 {
			fmt.Printf("    Port %d\n", conn.Port)
		}
		if conn.KeyName != "" {
			fmt.Printf("    IdentityFile %s\n", filepath.Join(config.KeysDir(), conn.KeyName))
		} else if conn.KeyPath != "" {
			fmt.Printf("    IdentityFile %s\n", conn.KeyPath)
		}
		if conn.JumpHost != "" {
			fmt.Printf("    ProxyJump %s\n", conn.JumpHost)
		}
		return nil
	},
}

var sshDescribeCmd = &cobra.Command{
	Use:   "describe <name>",
	Short: "Show system information of a remote host",
	Long: `Connect to a remote host and gather system information including
CPU, memory, disk, sysctl configuration, and running systemd services.
Assumes the user has sudo access; shows a notice if not.`,
	Example: `  spock ssh describe worker-1
  spock ssh describe worker-1 --user admin`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		conn, err := store.Get(name)
		if err != nil {
			return err
		}

		if u, _ := cmd.Flags().GetString("user"); u != "" {
			conn.User = u
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Gathering system info from %s@%s...\n\n", conn.User, conn.Host)

		password, _ := keyring.GetPassword(name)
		opts := sshclient.ConnectOptions{
			Host:     conn.Host,
			Port:     conn.Port,
			User:     conn.User,
			KeyName:  conn.KeyName,
			KeyPath:  conn.KeyPath,
			Password: password,
		}

		info, err := sshclient.DescribeNode(opts, cfg)
		if err != nil {
			return err
		}

		// Cache node info in the connection
		saveNodeInfo(name, info)

		fmt.Print(sshclient.FormatNodeInfo(info, name))
		return nil
	},
}

func saveNodeInfo(connName string, info *sshclient.NodeInfo) {
	conn, err := store.Get(connName)
	if err != nil {
		return
	}
	conn.NodeInfo = &store.CachedNodeInfo{
		Hostname:  info.Hostname,
		OS:        info.OS,
		Kernel:    info.Kernel,
		CPUModel:  info.CPUModel,
		CPUCores:  info.CPUCores,
		MemInfo:   info.MemInfo,
		DiskInfo:  info.DiskInfo,
		DiskTotal: info.DiskTotal,
		GPU:       info.GPU,
		HasSudo:   info.HasSudo,
		Sysctl:    info.Sysctl,
		Systemd:   info.Systemd,
		FetchedAt: time.Now(),
	}
	store.Update(conn)
}

func init() {
	rootCmd.AddCommand(sshCmd)

	sshCmd.AddCommand(sshToCmd)
	sshCmd.AddCommand(sshAddCmd)
	sshCmd.AddCommand(sshEditCmd)
	sshCmd.AddCommand(sshRemoveCmd)
	sshCmd.AddCommand(sshShowCmd)
	sshCmd.AddCommand(sshTestCmd)
	sshCmd.AddCommand(sshCopyCmd)
	sshCmd.AddCommand(sshImportCmd)
	sshCmd.AddCommand(sshExportCmd)
	sshCmd.AddCommand(sshDescribeCmd)

	// Flags for ssh to / ssh describe
	for _, c := range []*cobra.Command{sshToCmd, sshDescribeCmd} {
		c.Flags().StringP("user", "u", "", "Override SSH username")
	}

	// Flags for ssh add / ssh edit
	for _, c := range []*cobra.Command{sshAddCmd, sshEditCmd} {
		c.Flags().String("host", "", "Remote hostname or IP")
		c.Flags().StringP("user", "u", "", "SSH username")
		c.Flags().IntP("port", "p", 22, "SSH port")
		c.Flags().StringP("key", "k", "", "Spock-managed key name")
		c.Flags().String("key-path", "", "Path to an external SSH key")
		c.Flags().StringP("jump", "j", "", "Jump host (ProxyJump)")
		c.Flags().Bool("password", false, "Save password in OS keychain (prompts securely)")
	}
	sshEditCmd.Flags().Bool("clear-password", false, "Remove saved password from OS keychain")
}

// parseSSHConfig reads a basic ~/.ssh/config and returns connection profiles.
// Supports Host, HostName, User, Port, IdentityFile, ProxyJump.
func parseSSHConfig(path string) ([]*store.Connection, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var conns []*store.Connection
	var current *store.Connection
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first whitespace or '='
		var key, value string
		if idx := strings.IndexAny(line, " \t="); idx > 0 {
			key = line[:idx]
			value = strings.TrimLeft(line[idx:], " \t=")
		} else {
			continue
		}

		switch strings.ToLower(key) {
		case "host":
			if current != nil && current.Host != "" {
				conns = append(conns, current)
			}
			// Skip wildcards
			if strings.ContainsAny(value, "*?") {
				current = nil
				continue
			}
			current = &store.Connection{Name: value, Port: 22}
		case "hostname":
			if current != nil {
				current.Host = value
			}
		case "user":
			if current != nil {
				current.User = value
			}
		case "port":
			if current != nil {
				if p, err := strconv.Atoi(value); err == nil {
					current.Port = p
				}
			}
		case "identityfile":
			if current != nil {
				// Expand ~
				if strings.HasPrefix(value, "~/") {
					home, _ := os.UserHomeDir()
					value = filepath.Join(home, value[2:])
				}
				current.KeyPath = value
			}
		case "proxyjump":
			if current != nil {
				current.JumpHost = value
			}
		}
	}

	if current != nil && current.Host != "" {
		conns = append(conns, current)
	}
	return conns, scanner.Err()
}
