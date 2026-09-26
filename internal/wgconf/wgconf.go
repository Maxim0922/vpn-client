package wgconf

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

type Key string

const masked = "<redacted>"

func (k Key) Hex() (string, error) {
	raw, err := base64.StdEncoding.DecodeString(string(k))
	if err != nil {
		return "", fmt.Errorf("decode key: %w", err)
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("key must be 32 bytes, got %d", len(raw))
	}
	return hex.EncodeToString(raw), nil
}

type Interface struct {
	PrivateKey Key
	Address    []netip.Prefix
	DNS        []netip.Addr
	MTU        int
	ListenPort int
}

func (i Interface) HasIPv6() bool {
	for _, p := range i.Address {
		if p.Addr().Is6() && !p.Addr().Is4In6() {
			return true
		}
	}
	return false
}

type Peer struct {
	PublicKey    Key
	PresharedKey Key
	Endpoint     string
	AllowedIPs   []netip.Prefix
	Keepalive    int
}

type Config struct {
	Interface Interface
	Peers     []Peer
}

func Parse(text string) (*Config, error) {
	cfg := &Config{}
	var section string
	var peer *Peer

	lines := strings.Split(text, "\n")
	for n, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if section == "peer" {
				cfg.Peers = append(cfg.Peers, Peer{})
				peer = &cfg.Peers[len(cfg.Peers)-1]
			}
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value", n+1)
		}
		key = strings.ToLower(strings.TrimSpace(key))

		val = strings.TrimSpace(val)

		switch section {
		case "interface":
			if err := parseInterface(&cfg.Interface, key, val, n+1); err != nil {
				return nil, err
			}
		case "peer":
			if peer == nil {
				return nil, fmt.Errorf("line %d: peer key outside [Peer]", n+1)
			}
			if err := parsePeer(peer, key, val, n+1); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("line %d: key %q outside any section", n+1, key)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func parseInterface(iface *Interface, key, val string, ln int) error {
	switch key {
	case "privatekey":
		iface.PrivateKey = Key(val)
	case "address":
		ps, err := parsePrefixList(val)
		if err != nil {
			return fmt.Errorf("line %d: Address: %w", ln, err)
		}
		iface.Address = append(iface.Address, ps...)
	case "dns":
		for _, f := range splitList(val) {
			a, err := netip.ParseAddr(f)
			if err != nil {
				return fmt.Errorf("line %d: DNS %q: %w", ln, f, err)
			}
			iface.DNS = append(iface.DNS, a)
		}
	case "mtu":
		m, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("line %d: MTU: %w", ln, err)
		}
		iface.MTU = m
	case "listenport":
		p, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("line %d: ListenPort: %w", ln, err)
		}
		iface.ListenPort = p
	case "table", "preup", "postup", "predown", "postdown", "saveconfig":

	default:
		return fmt.Errorf("line %d: unknown Interface key %q", ln, key)
	}
	return nil
}

func parsePeer(p *Peer, key, val string, ln int) error {
	switch key {
	case "publickey":
		p.PublicKey = Key(val)
	case "presharedkey":
		p.PresharedKey = Key(val)
	case "endpoint":
		if _, _, err := net.SplitHostPort(val); err != nil {
			return fmt.Errorf("line %d: Endpoint %q: %w", ln, val, err)
		}
		p.Endpoint = val
	case "allowedips":
		ps, err := parsePrefixList(val)
		if err != nil {
			return fmt.Errorf("line %d: AllowedIPs: %w", ln, err)
		}
		p.AllowedIPs = append(p.AllowedIPs, ps...)
	case "persistentkeepalive":
		if val == "off" {
			p.Keepalive = 0
			return nil
		}
		k, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("line %d: PersistentKeepalive: %w", ln, err)
		}
		p.Keepalive = k
	default:
		return fmt.Errorf("line %d: unknown Peer key %q", ln, key)
	}
	return nil
}

func parsePrefixList(val string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range splitList(val) {
		p, err := netip.ParsePrefix(f)
		if err != nil {
			a, aerr := netip.ParseAddr(f)
			if aerr != nil {
				return nil, fmt.Errorf("%q: %w", f, err)
			}
			bits := 32
			if a.Is6() {
				bits = 128
			}
			p = netip.PrefixFrom(a, bits)
		}
		out = append(out, p)
	}
	return out, nil
}

func splitList(val string) []string {
	fields := strings.FieldsFunc(val, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	var out []string
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func (c *Config) validate() error {
	if c.Interface.PrivateKey == "" {
		return fmt.Errorf("missing Interface PrivateKey")
	}
	if _, err := c.Interface.PrivateKey.Hex(); err != nil {
		return fmt.Errorf("Interface PrivateKey: %w", err)
	}
	if len(c.Interface.Address) == 0 {
		return fmt.Errorf("missing Interface Address")
	}
	if len(c.Peers) == 0 {
		return fmt.Errorf("no peers")
	}
	for i := range c.Peers {
		p := &c.Peers[i]
		if p.PublicKey == "" {
			return fmt.Errorf("peer %d: missing PublicKey", i)
		}
		if _, err := p.PublicKey.Hex(); err != nil {
			return fmt.Errorf("peer %d PublicKey: %w", i, err)
		}
		if p.PresharedKey != "" {
			if _, err := p.PresharedKey.Hex(); err != nil {
				return fmt.Errorf("peer %d PresharedKey: %w", i, err)
			}
		}
		if p.Endpoint == "" {
			return fmt.Errorf("peer %d: missing Endpoint", i)
		}
	}
	return nil
}

func (c *Config) IPCSet() (string, error) {
	var b strings.Builder
	priv, err := c.Interface.PrivateKey.Hex()
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "private_key=%s\n", priv)
	if c.Interface.ListenPort != 0 {
		fmt.Fprintf(&b, "listen_port=%d\n", c.Interface.ListenPort)
	}
	b.WriteString("replace_peers=true\n")

	for i := range c.Peers {
		p := &c.Peers[i]
		pub, err := p.PublicKey.Hex()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "public_key=%s\n", pub)
		if p.PresharedKey != "" {
			psk, err := p.PresharedKey.Hex()
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "preshared_key=%s\n", psk)
		}
		fmt.Fprintf(&b, "endpoint=%s\n", p.Endpoint)
		if p.Keepalive != 0 {
			fmt.Fprintf(&b, "persistent_keepalive_interval=%d\n", p.Keepalive)
		}
		b.WriteString("replace_allowed_ips=true\n")
		for _, a := range p.AllowedIPs {
			fmt.Fprintf(&b, "allowed_ip=%s\n", a.String())
		}
	}
	return b.String(), nil
}

func (c *Config) Endpoint() (host, port string, err error) {
	if len(c.Peers) == 0 {
		return "", "", fmt.Errorf("no peers")
	}
	return net.SplitHostPort(c.Peers[0].Endpoint)
}

func (c Config) String() string {
	c.Interface.PrivateKey = masked
	c.Peers = append([]Peer(nil), c.Peers...)
	for i := range c.Peers {
		c.Peers[i].PublicKey = maskKey(c.Peers[i].PublicKey)
		if c.Peers[i].PresharedKey != "" {
			c.Peers[i].PresharedKey = masked
		}
	}
	return fmt.Sprintf("Config{iface:%+v peers:%+v}", c.Interface, c.Peers)
}

func maskKey(k Key) Key {
	s := string(k)
	if len(s) <= 8 {
		return masked
	}
	return Key(s[:8] + "…")
}
