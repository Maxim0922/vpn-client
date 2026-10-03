package openvpn

import (
	"bufio"
	"context"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	RemoteHost string
	RemotePort string
	Proto      string
	DNS        []netip.Addr
	Dev        string
	HasIPv6    bool
	Raw        string
}

func (c *Config) Endpoint() (host, port, proto string, err error) {
	if c.RemoteHost == "" {
		return "", "", "", fmt.Errorf("missing remote in openvpn config")
	}
	port = c.RemotePort
	if port == "" {
		port = "1194"
	}
	proto = c.Proto
	if proto == "" {
		proto = "udp"
	}
	return c.RemoteHost, port, proto, nil
}

func (c *Config) String() string {
	proto := c.Proto
	if proto == "" {
		proto = "udp"
	}
	port := c.RemotePort
	if port == "" {
		port = "1194"
	}
	return fmt.Sprintf("openvpn %s:%s proto=%s", c.RemoteHost, port, proto)
}

func IsConfig(text string) bool {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "vless://") {
		return false
	}
	if strings.Contains(trimmed, "[Interface]") || strings.Contains(trimmed, "[Peer]") {
		return false
	}

	sc := bufio.NewScanner(strings.NewReader(trimmed))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		lower := strings.ToLower(line)
		fields := strings.Fields(lower)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "client", "tls-client", "remote", "dev", "dev-type", "proto",
			"resolv-retry", "nobind", "persist-key", "persist-tun", "ca",
			"cert", "key", "tls-auth", "tls-crypt", "auth-user-pass",
			"cipher", "data-ciphers", "comp-lzo":
			return true
		}
		if strings.HasPrefix(lower, "<ca>") || strings.HasPrefix(lower, "<cert>") ||
			strings.HasPrefix(lower, "<key>") || strings.HasPrefix(lower, "<tls-auth>") ||
			strings.HasPrefix(lower, "<tls-crypt>") {
			return true
		}
	}
	return false
}

func Parse(text string) (*Config, error) {
	cfg := &Config{
		RemotePort: "1194",
		Proto:      "udp",
		Dev:        "tun",
		Raw:        text,
	}

	sc := bufio.NewScanner(strings.NewReader(text))
	inTag := false
	var currentTag string

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if inTag {
			if strings.HasPrefix(strings.ToLower(line), "</"+currentTag+">") {
				inTag = false
				currentTag = ""
			}
			continue
		}
		if strings.HasPrefix(line, "<") && strings.Contains(line, ">") {
			tag := strings.ToLower(line[1:strings.Index(line, ">")])
			tag = strings.TrimSpace(tag)
			if !strings.HasSuffix(strings.ToLower(line), "</"+tag+">") {
				inTag = true
				currentTag = tag
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		cmd := strings.ToLower(fields[0])

		switch cmd {
		case "remote":
			if len(fields) >= 2 && cfg.RemoteHost == "" {
				cfg.RemoteHost = fields[1]
			}
			if len(fields) >= 3 {
				cfg.RemotePort = fields[2]
			}
			if len(fields) >= 4 {
				cfg.Proto = normalizeProto(fields[3])
			}
		case "port", "rport":
			if len(fields) >= 2 {
				cfg.RemotePort = fields[1]
			}
		case "proto":
			if len(fields) >= 2 {
				cfg.Proto = normalizeProto(fields[1])
			}
		case "dev":
			if len(fields) >= 2 {
				cfg.Dev = fields[1]
			}
		case "dhcp-option":
			if len(fields) >= 3 && strings.EqualFold(fields[1], "DNS") {
				if addr, err := netip.ParseAddr(fields[2]); err == nil {
					cfg.DNS = append(cfg.DNS, addr)
				}
			}
		case "tun-ipv6", "ifconfig-ipv6", "route-ipv6":
			cfg.HasIPv6 = true
		}
	}

	if cfg.RemoteHost == "" {
		return nil, fmt.Errorf("openvpn config missing 'remote' host")
	}
	if _, err := strconv.Atoi(cfg.RemotePort); err != nil {
		return nil, fmt.Errorf("invalid openvpn remote port %q", cfg.RemotePort)
	}

	return cfg, nil
}

func normalizeProto(p string) string {
	p = strings.ToLower(p)
	switch {
	case strings.HasPrefix(p, "tcp"):
		return "tcp"
	default:
		return "udp"
	}
}

var DefaultBinaryPaths = []string{
	"/opt/homebrew/sbin/openvpn",
	"/opt/homebrew/bin/openvpn",
	"/usr/local/sbin/openvpn",
	"/usr/local/bin/openvpn",
	"/usr/sbin/openvpn",
	"/usr/bin/openvpn",
}

func FindBinary() (string, error) {
	for _, p := range DefaultBinaryPaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0111 != 0 {
			return p, nil
		}
	}
	if p, err := exec.LookPath("openvpn"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("openvpn binary not found: please install openvpn (e.g. 'brew install openvpn')")
}

type Tunnel struct {
	name      string
	cmd       *exec.Cmd
	mu        sync.Mutex
	lastAlive time.Time
	ready     chan struct{}
	errChan   chan error
	closed    bool
}

func (t *Tunnel) Name() string { return t.name }

func (t *Tunnel) LastAlive() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastAlive
}

func (t *Tunnel) TouchAlive() {
	t.mu.Lock()
	t.lastAlive = time.Now()
	t.mu.Unlock()
}

func (t *Tunnel) Err() error {
	select {
	case err := <-t.errChan:
		t.errChan <- err
		return err
	default:
		return nil
	}
}

func (t *Tunnel) WaitReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-t.errChan:
		return err
	case <-t.ready:
		return nil
	}
}

func (t *Tunnel) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	cmd := t.cmd
	t.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
	return nil
}

func Start(ctx context.Context, cfg *Config, confPath string, iface string, log func(string)) (*Tunnel, error) {
	bin, err := FindBinary()
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = func(string) {}
	}

	args := []string{
		"--config", confPath,
		"--dev", iface,
		"--dev-type", "tun",
		"--nobind",
		"--route-noexec",
		"--suppress-timestamps",
	}

	cmd := exec.Command(bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("openvpn stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	t := &Tunnel{
		name:    iface,
		cmd:     cmd,
		ready:   make(chan struct{}),
		errChan: make(chan error, 1),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start openvpn (%s): %w", bin, err)
	}

	var readyOnce sync.Once
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			line := sc.Text()
			log("openvpn: " + line)

			if strings.Contains(line, "Initialization Sequence Completed") {
				t.TouchAlive()
				readyOnce.Do(func() {
					close(t.ready)
				})
			}
			if strings.Contains(line, "AUTH_FAILED") {
				t.errChan <- fmt.Errorf("openvpn authentication failed")
			}
		}
		if err := sc.Err(); err != nil {
			log(fmt.Sprintf("openvpn read error: %v", err))
		}
		cmdErr := cmd.Wait()
		readyOnce.Do(func() {
			if cmdErr != nil {
				t.errChan <- fmt.Errorf("openvpn exited prematurely: %w", cmdErr)
			} else {
				t.errChan <- fmt.Errorf("openvpn exited unexpectedly")
			}
		})
	}()

	return t, nil
}
