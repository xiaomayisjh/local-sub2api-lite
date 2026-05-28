package localconfig

import (
	"net"
	"path/filepath"
	"strconv"
	"testing"
)

func TestIsPortAvailable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi: %v", err)
	}

	if IsPortAvailable("127.0.0.1", port) {
		t.Fatalf("expected port %d to be unavailable", port)
	}
	if IsPortAvailable("127.0.0.1", 0) {
		t.Fatalf("invalid port should not be available")
	}
}

func TestFindAvailablePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	busy, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi: %v", err)
	}

	got, err := FindAvailablePort("127.0.0.1", busy, 5)
	if err != nil {
		t.Fatalf("FindAvailablePort: %v", err)
	}
	if got == busy {
		t.Fatalf("expected different port, got %d", got)
	}
	if !IsPortAvailable("127.0.0.1", got) {
		t.Fatalf("expected port %d to be available", got)
	}
}

func TestUpdateServerPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := writeFile(path, []byte("server:\n  host: 127.0.0.1\n  port: 8080\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := UpdateServerPort(path, 9090); err != nil {
		t.Fatalf("UpdateServerPort: %v", err)
	}
	doc, err := readYAMLFile(path)
	if err != nil {
		t.Fatalf("readYAMLFile: %v", err)
	}
	server, _ := doc["server"].(map[string]any)
	if server["port"] != 9090 {
		t.Fatalf("port = %v, want 9090", server["port"])
	}
}
