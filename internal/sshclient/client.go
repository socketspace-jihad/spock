package sshclient

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"

	"github.com/muhammadjihad/spock/internal/config"
)

type ConnectOptions struct {
	Host     string
	Port     int
	User     string
	KeyName  string
	KeyPath  string
	Password string // from OS keychain, if available
}

// Connect opens an interactive SSH session. output receives all session output
// (typically a Broadcaster that fans out to terminal + log + mirror sockets).
func Connect(opts ConnectOptions, output io.Writer, cfg *config.AppConfig) error {
	if opts.Port == 0 {
		opts.Port = 22
	}

	authMethods := buildAuthMethods(opts, cfg)
	sshConfig := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session failed: %w", err)
	}
	defer session.Close()

	// Put terminal in raw mode
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("terminal setup failed: %w", err)
	}
	defer term.Restore(fd, oldState)

	w, h, _ := term.GetSize(fd)
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", h, w, modes); err != nil {
		return fmt.Errorf("PTY request failed: %w", err)
	}

	// Window resize handler
	resizeCh := make(chan os.Signal, 1)
	registerSigwinch(resizeCh)
	go func() {
		for range resizeCh {
			if nw, nh, err := term.GetSize(fd); err == nil {
				_ = session.WindowChange(nh, nw)
			}
		}
	}()
	defer stopSigwinch(resizeCh)

	// Set up I/O pipes
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return err
	}
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(output, stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		io.Copy(output, stderrPipe)
	}()
	go func() {
		io.Copy(stdinPipe, os.Stdin)
		stdinPipe.Close()
	}()

	if err := session.Shell(); err != nil {
		return fmt.Errorf("shell failed: %w", err)
	}

	err = session.Wait()
	wg.Wait()
	return err
}

// TestConnection verifies that an SSH connection can be established.
func TestConnection(opts ConnectOptions, cfg *config.AppConfig) (time.Duration, error) {
	if opts.Port == 0 {
		opts.Port = 22
	}

	authMethods := buildAuthMethods(opts, cfg)
	sshConfig := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	start := time.Now()
	client, err := ssh.Dial("tcp", addr, sshConfig)
	elapsed := time.Since(start)
	if err != nil {
		return elapsed, err
	}
	client.Close()
	return elapsed, nil
}

// RunCommand executes a single command over SSH and returns the output.
func RunCommand(opts ConnectOptions, command string, cfg *config.AppConfig) (string, error) {
	if opts.Port == 0 {
		opts.Port = 22
	}

	authMethods := buildAuthMethods(opts, cfg)
	sshConfig := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	return string(output), err
}

func buildAuthMethods(opts ConnectOptions, cfg *config.AppConfig) []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	// 1. SSH agent
	if cfg.UseSSHAgent {
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			conn, err := net.Dial("unix", sock)
			if err == nil {
				agentClient := agent.NewClient(conn)
				methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
			}
		}
	}

	// 2. Connection-specific key by name (from ~/.spock/keys/)
	if opts.KeyName != "" {
		keyPath := filepath.Join(config.KeysDir(), opts.KeyName)
		if signer, err := loadSigner(keyPath); err == nil {
			methods = append(methods, ssh.PublicKeys(signer))
		}
	}

	// 3. Connection-specific key by path
	if opts.KeyPath != "" {
		if signer, err := loadSigner(opts.KeyPath); err == nil {
			methods = append(methods, ssh.PublicKeys(signer))
		}
	}

	// 4. Default SSH keys
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		kp := filepath.Join(home, ".ssh", name)
		if signer, err := loadSigner(kp); err == nil {
			methods = append(methods, ssh.PublicKeys(signer))
		}
	}

	// 5. Stored password (from OS keychain)
	if opts.Password != "" {
		methods = append(methods, ssh.Password(opts.Password))
	}

	// 6. Interactive password prompt (last resort)
	methods = append(methods, ssh.PasswordCallback(func() (string, error) {
		fmt.Fprintf(os.Stderr, "Password for %s@%s: ", opts.User, opts.Host)
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(pw), err
	}))

	return methods
}

func loadSigner(keyPath string) (ssh.Signer, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		var passErr *ssh.PassphraseMissingError
		if errors.As(err, &passErr) {
			fmt.Fprintf(os.Stderr, "Passphrase for %s: ", filepath.Base(keyPath))
			passphrase, perr := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if perr != nil {
				return nil, perr
			}
			return ssh.ParsePrivateKeyWithPassphrase(data, passphrase)
		}
		return nil, err
	}
	return signer, nil
}

func hostKeyCallback() ssh.HostKeyCallback {
	home, _ := os.UserHomeDir()
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	cb, err := knownhosts.New(knownHostsPath)
	if err != nil {
		// If known_hosts doesn't exist, ensure ~/.ssh exists and create it
		sshDir := filepath.Join(home, ".ssh")
		os.MkdirAll(sshDir, 0700)
		os.WriteFile(knownHostsPath, []byte{}, 0644)
		cb, err = knownhosts.New(knownHostsPath)
		if err != nil {
			return ssh.InsecureIgnoreHostKey()
		}
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}

		fp := ssh.FingerprintSHA256(key)
		reader := bufio.NewReader(os.Stdin)

		if len(keyErr.Want) == 0 {
			// Unknown host — TOFU prompt
			fmt.Fprintf(os.Stderr, "\nHost %q is not in known_hosts.\n", hostname)
			fmt.Fprintf(os.Stderr, "%s key fingerprint: %s\n", key.Type(), fp)
			fmt.Fprintf(os.Stderr, "Trust this host? (yes/no): ")
		} else {
			// Host key CHANGED — warn and ask to update
			fmt.Fprintf(os.Stderr, "\n@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n")
			fmt.Fprintf(os.Stderr, "@ WARNING: HOST KEY HAS CHANGED!        @\n")
			fmt.Fprintf(os.Stderr, "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n")
			fmt.Fprintf(os.Stderr, "Host:        %s\n", hostname)
			fmt.Fprintf(os.Stderr, "New %s fingerprint: %s\n", key.Type(), fp)
			fmt.Fprintf(os.Stderr, "\nThe host key has changed. This could mean the server was\n")
			fmt.Fprintf(os.Stderr, "reinstalled, or it could indicate a man-in-the-middle attack.\n")
			fmt.Fprintf(os.Stderr, "Accept the new key and update known_hosts? (yes/no): ")
		}

		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))

		if answer != "yes" && answer != "y" {
			return fmt.Errorf("host key rejected by user")
		}

		// Remove old entries for this host, then add the new key
		if len(keyErr.Want) > 0 {
			removeHostFromKnownHosts(knownHostsPath, hostname)
		}

		line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
		f, ferr := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if ferr == nil {
			fmt.Fprintln(f, line)
			f.Close()
		}
		return nil
	}
}

// removeHostFromKnownHosts removes all entries matching the given hostname
// from the known_hosts file so a new key can be added.
func removeHostFromKnownHosts(path, hostname string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	normalized := knownhosts.Normalize(hostname)
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			kept = append(kept, line)
			continue
		}
		// Each line starts with hostnames separated by commas, then a space
		fields := strings.SplitN(trimmed, " ", 2)
		if len(fields) < 2 {
			kept = append(kept, line)
			continue
		}
		hosts := strings.Split(fields[0], ",")
		match := false
		for _, h := range hosts {
			if strings.TrimSpace(h) == normalized || strings.TrimSpace(h) == hostname {
				match = true
				break
			}
		}
		if !match {
			kept = append(kept, line)
		}
	}
	os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0644)
}

func registerSigwinch(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.Signal(0x1c)) // SIGWINCH
}

func stopSigwinch(ch chan<- os.Signal) {
	signal.Stop(ch)
}
