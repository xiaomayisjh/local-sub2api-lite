package localconfig

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultHTTPPort is the default local desktop HTTP port.
	DefaultHTTPPort = 8080
	// MaxPortSearchAttempts limits how many consecutive ports to try on conflict.
	MaxPortSearchAttempts = 128
)

// IsPortAvailable reports whether host:port can be bound for TCP listen.
func IsPortAvailable(host string, port int) bool {
	if port < 1 || port > 65535 {
		return false
	}
	if host == "" {
		host = "127.0.0.1"
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// FindAvailablePort returns the first available port in [start, start+maxAttempts).
func FindAvailablePort(host string, start, maxAttempts int) (int, error) {
	if start < 1 {
		start = DefaultHTTPPort
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for i := 0; i < maxAttempts; i++ {
		port := start + i
		if port > 65535 {
			break
		}
		if IsPortAvailable(host, port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found from %d within %d attempts", start, maxAttempts)
}

// ResolveListenPort returns a port to bind. When preferred is busy and autoFallback is true,
// the next available port is chosen and written to configPath.
func ResolveListenPort(host string, preferred int, autoFallback bool, configPath string) (port int, switchedFrom int, err error) {
	if host == "" {
		host = "127.0.0.1"
	}
	if preferred < 1 || preferred > 65535 {
		preferred = DefaultHTTPPort
	}
	if IsPortAvailable(host, preferred) {
		return preferred, 0, nil
	}
	if !autoFallback {
		return 0, preferred, fmt.Errorf("port %d is already in use on %s", preferred, host)
	}
	alt, findErr := FindAvailablePort(host, preferred, MaxPortSearchAttempts)
	if findErr != nil {
		return 0, preferred, fmt.Errorf("port %d is in use on %s and no fallback port was found", preferred, host)
	}
	if configPath != "" {
		if err := UpdateServerPort(configPath, alt); err != nil {
			return 0, preferred, err
		}
	}
	return alt, preferred, nil
}

// UpdateServerPort updates only server.port in a YAML config file.
func UpdateServerPort(configPath string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	doc, err := readYAMLFile(configPath)
	if err != nil {
		return err
	}
	server, _ := doc["server"].(map[string]any)
	if server == nil {
		server = map[string]any{}
		doc["server"] = server
	}
	server["port"] = port
	return writeYAMLFile(configPath, doc)
}

// WaitForPortAvailable polls until the port becomes available or timeout elapses.
func WaitForPortAvailable(host string, port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsPortAvailable(host, port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return IsPortAvailable(host, port)
}

func readYAMLFile(path string) (map[string]any, error) {
	raw, err := readFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func writeYAMLFile(path string, doc map[string]any) error {
	raw, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return writeFile(path, raw, 0o600)
}
