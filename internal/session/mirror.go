package session

import (
	"fmt"
	"io"
	"net"
	"os"
)

// MirrorServer listens on a Unix socket and accepts read-only mirror clients.
type MirrorServer struct {
	listener    net.Listener
	broadcaster *Broadcaster
	socketPath  string
	done        chan struct{}
}

func StartMirrorServer(socketPath string, broadcaster *Broadcaster) (*MirrorServer, error) {
	// Remove stale socket if it exists
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("mirror server failed: %w", err)
	}

	srv := &MirrorServer{
		listener:    listener,
		broadcaster: broadcaster,
		socketPath:  socketPath,
		done:        make(chan struct{}),
	}

	go srv.acceptLoop()
	return srv, nil
}

func (s *MirrorServer) acceptLoop() {
	defer close(s.done)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // Listener closed
		}
		s.broadcaster.AddListener(conn)
	}
}

func (s *MirrorServer) Close() {
	s.listener.Close()
	os.Remove(s.socketPath)
	<-s.done
}

// MirrorConnect connects to an active session's mirror socket and streams output.
func MirrorConnect(socketPath string, output io.Writer) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("cannot connect to session: %w", err)
	}
	defer conn.Close()

	_, err = io.Copy(output, conn)
	return err
}
