package session

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/muhammadjihad/spock/internal/config"
)

type SessionInfo struct {
	ID             string    `json:"id"`
	ConnectionName string    `json:"connection_name"`
	Host           string    `json:"host"`
	User           string    `json:"user"`
	PID            int       `json:"pid"`
	StartedAt      time.Time `json:"started_at"`
	LogFile        string    `json:"log_file"`
	SocketPath     string    `json:"socket_path"`
}

type activeStore struct {
	Sessions map[string]*SessionInfo `json:"sessions"`
}

var adjectives = []string{
	"brave", "calm", "cool", "dark", "fair", "fast", "free", "glad",
	"gold", "gray", "green", "keen", "kind", "lean", "loud", "mild",
	"neat", "pale", "pure", "rare", "rich", "safe", "slim", "soft",
	"tall", "tame", "warm", "wild", "wise", "bold",
}

var nouns = []string{
	"ant", "bear", "cat", "crane", "crow", "deer", "dove", "eagle",
	"elk", "fox", "frog", "goat", "hawk", "heron", "jay", "lark",
	"lion", "lynx", "mole", "moth", "orca", "owl", "puma", "raven",
	"seal", "shark", "swan", "tiger", "wolf", "wren",
}

func generateID() string {
	ai, _ := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	ni, _ := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	return adjectives[ai.Int64()] + "-" + nouns[ni.Int64()]
}

func loadActive() (*activeStore, error) {
	data, err := os.ReadFile(config.ActiveSessionsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &activeStore{Sessions: make(map[string]*SessionInfo)}, nil
		}
		return nil, err
	}
	var s activeStore
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Sessions == nil {
		s.Sessions = make(map[string]*SessionInfo)
	}
	return &s, nil
}

func saveActive(s *activeStore) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.ActiveSessionsPath(), data, 0644)
}

func Register(connName, host, user string) (*SessionInfo, error) {
	if err := config.EnsureDirs(); err != nil {
		return nil, err
	}

	s, err := loadActive()
	if err != nil {
		return nil, err
	}

	// Generate unique ID
	id := generateID()
	for _, exists := s.Sessions[id]; exists; _, exists = s.Sessions[id] {
		id = generateID()
	}

	now := time.Now()
	logFile := filepath.Join(config.LogsDir(), fmt.Sprintf("%s_%s.log", connName, now.Format("2006-01-02T15-04-05")))
	socketPath := filepath.Join(config.SocketsDir(), id+".sock")

	info := &SessionInfo{
		ID:             id,
		ConnectionName: connName,
		Host:           host,
		User:           user,
		PID:            os.Getpid(),
		StartedAt:      now,
		LogFile:        logFile,
		SocketPath:     socketPath,
	}

	s.Sessions[id] = info
	if err := saveActive(s); err != nil {
		return nil, err
	}
	return info, nil
}

func Deregister(id string) {
	s, err := loadActive()
	if err != nil {
		return
	}
	info, ok := s.Sessions[id]
	if ok {
		os.Remove(info.SocketPath)
		delete(s.Sessions, id)
		saveActive(s)
	}
}

func ListSessions() ([]*SessionInfo, error) {
	s, err := loadActive()
	if err != nil {
		return nil, err
	}
	var sessions []*SessionInfo
	for _, info := range s.Sessions {
		sessions = append(sessions, info)
	}
	return sessions, nil
}

func GetSession(idOrName string) (*SessionInfo, error) {
	s, err := loadActive()
	if err != nil {
		return nil, err
	}

	// Try by ID first
	if info, ok := s.Sessions[idOrName]; ok {
		return info, nil
	}

	// Try by connection name (return first match)
	for _, info := range s.Sessions {
		if info.ConnectionName == idOrName {
			return info, nil
		}
	}

	return nil, fmt.Errorf("session %q not found", idOrName)
}
