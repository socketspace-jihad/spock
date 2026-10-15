package cmd

import (
	"fmt"
	"os"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/muhammadjihad/spock/internal/session"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage active SSH sessions",
	Long:  "List, mirror, kill, and inspect active SSH sessions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Silently clean stale sessions first
		session.CleanStale()

		sessions, err := session.ListSessions()
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			fmt.Println("No active sessions.")
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tCONNECTION\tHOST\tUSER\tSTARTED\tPID")
		for _, s := range sessions {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n",
				s.ID,
				s.ConnectionName,
				s.Host,
				s.User,
				s.StartedAt.Format("15:04:05"),
				s.PID,
			)
		}
		return tw.Flush()
	},
}

var sessionMirrorCmd = &cobra.Command{
	Use:   "mirror <session-id|connection-name>",
	Short: "Watch an active session (read-only)",
	Long: `Mirror streams the output of an active SSH session to your terminal.
You can reference sessions by their human-readable ID (e.g., calm-tiger)
or by the connection name if there is only one active session for it.

Press Ctrl+C to stop mirroring (the original session continues).`,
	Example: `  spock session mirror calm-tiger
  spock session mirror worker-1`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		session.CleanStale()

		sess, err := session.GetSession(args[0])
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Mirroring session %s (%s@%s) — press Ctrl+C to stop\n",
			sess.ID, sess.User, sess.Host)

		return session.MirrorConnect(sess.SocketPath, os.Stdout)
	},
}

var sessionKillCmd = &cobra.Command{
	Use:   "kill <session-id|connection-name>",
	Short: "Terminate an active session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sess, err := session.GetSession(args[0])
		if err != nil {
			return err
		}

		process, err := os.FindProcess(sess.PID)
		if err != nil {
			return fmt.Errorf("process %d not found", sess.PID)
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to kill session: %w", err)
		}

		session.Deregister(sess.ID)
		fmt.Printf("Session %s killed.\n", sess.ID)
		return nil
	},
}

var sessionLogCmd = &cobra.Command{
	Use:   "log <session-id|connection-name>",
	Short: "Show the transcript of a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sess, err := session.GetSession(args[0])
		if err != nil {
			return err
		}

		data, err := os.ReadFile(sess.LogFile)
		if err != nil {
			return fmt.Errorf("log file not found: %w", err)
		}
		os.Stdout.Write(data)
		return nil
	},
}

var sessionCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove stale session records and orphaned sockets",
	RunE: func(cmd *cobra.Command, args []string) error {
		removed, err := session.CleanStale()
		if err != nil {
			return err
		}
		if removed == 0 {
			fmt.Println("No stale sessions found.")
		} else {
			fmt.Printf("Cleaned up %d stale session(s).\n", removed)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionMirrorCmd)
	sessionCmd.AddCommand(sessionKillCmd)
	sessionCmd.AddCommand(sessionLogCmd)
	sessionCmd.AddCommand(sessionCleanCmd)
}
