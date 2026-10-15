package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/muhammadjihad/spock/internal/config"
)

type CachedNodeInfo struct {
	Hostname  string    `json:"hostname,omitempty"`
	OS        string    `json:"os,omitempty"`
	Kernel    string    `json:"kernel,omitempty"`
	CPUModel  string    `json:"cpu_model,omitempty"`
	CPUCores  string    `json:"cpu_cores,omitempty"`
	MemInfo   string    `json:"mem_info,omitempty"`
	DiskInfo  string    `json:"disk_info,omitempty"`
	DiskTotal string    `json:"disk_total,omitempty"`
	GPU       string    `json:"gpu,omitempty"`
	HasSudo   bool      `json:"has_sudo,omitempty"`
	Sysctl    string    `json:"sysctl,omitempty"`
	Systemd   string    `json:"systemd,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

type Connection struct {
	Name          string          `json:"name"`
	Host          string          `json:"host"`
	Port          int             `json:"port"`
	User          string          `json:"user"`
	Users         []string        `json:"users,omitempty"`
	KeyName       string          `json:"key_name,omitempty"`
	KeyPath       string          `json:"key_path,omitempty"`
	JumpHost      string          `json:"jump_host,omitempty"`
	HasPassword   bool            `json:"has_password,omitempty"`
	NodeInfo      *CachedNodeInfo `json:"node_info,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	LastConnected *time.Time      `json:"last_connected,omitempty"`
}

type connectionStore struct {
	Version     int                    `json:"version"`
	Connections map[string]*Connection `json:"connections"`
}

func loadStore() (*connectionStore, error) {
	data, err := os.ReadFile(config.ConnectionsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &connectionStore{Version: 1, Connections: make(map[string]*Connection)}, nil
		}
		return nil, err
	}
	var s connectionStore
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Connections == nil {
		s.Connections = make(map[string]*Connection)
	}
	return &s, nil
}

func saveStore(s *connectionStore) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.ConnectionsPath(), data, 0644)
}

func Add(conn *Connection) error {
	s, err := loadStore()
	if err != nil {
		return err
	}
	if _, exists := s.Connections[conn.Name]; exists {
		return fmt.Errorf("connection %q already exists", conn.Name)
	}
	conn.CreatedAt = time.Now()
	if conn.Port == 0 {
		conn.Port = 22
	}
	s.Connections[conn.Name] = conn
	return saveStore(s)
}

func Get(name string) (*Connection, error) {
	s, err := loadStore()
	if err != nil {
		return nil, err
	}
	conn, ok := s.Connections[name]
	if !ok {
		return nil, fmt.Errorf("connection %q not found", name)
	}
	return conn, nil
}

func Update(conn *Connection) error {
	s, err := loadStore()
	if err != nil {
		return err
	}
	if _, exists := s.Connections[conn.Name]; !exists {
		return fmt.Errorf("connection %q not found", conn.Name)
	}
	s.Connections[conn.Name] = conn
	return saveStore(s)
}

func Remove(name string) error {
	s, err := loadStore()
	if err != nil {
		return err
	}
	if _, exists := s.Connections[name]; !exists {
		return fmt.Errorf("connection %q not found", name)
	}
	delete(s.Connections, name)
	return saveStore(s)
}

func List() ([]*Connection, error) {
	s, err := loadStore()
	if err != nil {
		return nil, err
	}
	conns := make([]*Connection, 0, len(s.Connections))
	for _, c := range s.Connections {
		conns = append(conns, c)
	}
	sort.Slice(conns, func(i, j int) bool { return conns[i].Name < conns[j].Name })
	return conns, nil
}

func Touch(name string) {
	s, err := loadStore()
	if err != nil {
		return
	}
	conn, ok := s.Connections[name]
	if !ok {
		return
	}
	now := time.Now()
	conn.LastConnected = &now
	saveStore(s)
}

func Copy(src, dst string) error {
	s, err := loadStore()
	if err != nil {
		return err
	}
	srcConn, ok := s.Connections[src]
	if !ok {
		return fmt.Errorf("connection %q not found", src)
	}
	if _, exists := s.Connections[dst]; exists {
		return fmt.Errorf("connection %q already exists", dst)
	}
	newConn := *srcConn
	newConn.Name = dst
	newConn.CreatedAt = time.Now()
	newConn.LastConnected = nil
	s.Connections[dst] = &newConn
	return saveStore(s)
}

func SaveNodeInfo(name string, info *CachedNodeInfo) error {
	s, err := loadStore()
	if err != nil {
		return err
	}
	conn, ok := s.Connections[name]
	if !ok {
		return fmt.Errorf("connection %q not found", name)
	}
	info.FetchedAt = time.Now()
	conn.NodeInfo = info
	return saveStore(s)
}

func ImportBatch(conns []*Connection) (added, skipped int, err error) {
	s, err := loadStore()
	if err != nil {
		return 0, 0, err
	}
	for _, conn := range conns {
		if _, exists := s.Connections[conn.Name]; exists {
			skipped++
			continue
		}
		conn.CreatedAt = time.Now()
		if conn.Port == 0 {
			conn.Port = 22
		}
		s.Connections[conn.Name] = conn
		added++
	}
	if added > 0 {
		err = saveStore(s)
	}
	return added, skipped, err
}
