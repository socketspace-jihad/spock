package session

import (
	"io"
	"net"
	"sync"
	"time"
)

const ringSize = 4096

// Broadcaster is an io.Writer that fans out writes to:
//   - the primary terminal (always)
//   - a log file (if provided)
//   - all connected mirror clients (via Unix sockets)
//
// It also keeps a ring buffer of recent output so late-joining
// mirror clients get context instead of a blank screen.
type Broadcaster struct {
	mu        sync.RWMutex
	terminal  io.Writer
	logFile   io.Writer
	listeners map[net.Conn]struct{}
	ring      []byte
	ringPos   int
	ringFull  bool
}

func NewBroadcaster(terminal io.Writer, logFile io.Writer) *Broadcaster {
	return &Broadcaster{
		terminal:  terminal,
		logFile:   logFile,
		listeners: make(map[net.Conn]struct{}),
		ring:      make([]byte, ringSize),
	}
}

func (b *Broadcaster) Write(p []byte) (int, error) {
	// Always write to terminal
	n, err := b.terminal.Write(p)
	if err != nil {
		return n, err
	}

	// Write to log (best effort)
	if b.logFile != nil {
		b.logFile.Write(p)
	}

	// Update ring buffer
	b.mu.Lock()
	for _, byt := range p {
		b.ring[b.ringPos] = byt
		b.ringPos = (b.ringPos + 1) % ringSize
		if b.ringPos == 0 {
			b.ringFull = true
		}
	}

	// Broadcast to mirror clients (best effort, non-blocking)
	for conn := range b.listeners {
		conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		if _, werr := conn.Write(p); werr != nil {
			conn.Close()
			delete(b.listeners, conn)
		}
	}
	b.mu.Unlock()

	return n, nil
}

func (b *Broadcaster) AddListener(conn net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Send ring buffer contents for context
	catchUp := b.RingBytes()
	if len(catchUp) > 0 {
		conn.SetWriteDeadline(time.Now().Add(time.Second))
		conn.Write(catchUp)
	}

	b.listeners[conn] = struct{}{}
}

func (b *Broadcaster) RemoveListener(conn net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.listeners, conn)
}

// RingBytes returns the current ring buffer contents in order.
func (b *Broadcaster) RingBytes() []byte {
	if !b.ringFull {
		return append([]byte{}, b.ring[:b.ringPos]...)
	}
	result := make([]byte, ringSize)
	copy(result, b.ring[b.ringPos:])
	copy(result[ringSize-b.ringPos:], b.ring[:b.ringPos])
	return result
}

func (b *Broadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for conn := range b.listeners {
		conn.Close()
		delete(b.listeners, conn)
	}
}
