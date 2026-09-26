package vless

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	Name string
	UUID string
	Host string
	Port uint16
	Flow string

	Security    string
	SNI         string
	Fingerprint string
	ALPN        []string
	Insecure    bool
	PublicKey   string
	ShortID     string

	Transport   string
	Path        string
	HostHeader  string
	ServiceName string
	EarlyData   uint32
}

var TunAddress = netip.MustParsePrefix("172.19.0.1/30")

type Options struct {
	Iface    string
	MTU      int
	ServerIP string
	DNS      netip.Addr
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsLink(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "vless://")
}

func Parse(link string) (*Config, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return nil, fmt.Errorf("parse link: %w", err)
	}
	if u.Scheme != "vless" {
		return nil, fmt.Errorf("not a vless:// link")
	}
	c := &Config{Name: u.Fragment, UUID: u.User.Username(), Host: u.Hostname()}
	if !uuidRe.MatchString(c.UUID) {
		return nil, fmt.Errorf("invalid uuid %q", c.UUID)
	}
	if c.Host == "" {
		return nil, fmt.Errorf("missing server host")
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil || port == 0 {
		return nil, fmt.Errorf("invalid port %q", u.Port())
	}
	c.Port = uint16(port)

	q := u.Query()
	if enc := q.Get("encryption"); enc != "" && enc != "none" {
		return nil, fmt.Errorf("encryption %q not supported (only none)", enc)
	}
	c.Flow = q.Get("flow")
	if c.Flow != "" && c.Flow != "xtls-rprx-vision" {
		return nil, fmt.Errorf("flow %q not supported", c.Flow)
	}

	c.Security = strings.ToLower(q.Get("security"))
	switch c.Security {
	case "", "none":
		c.Security = "none"
	case "tls", "reality":
	default:
		return nil, fmt.Errorf("security %q not supported", c.Security)
	}
	c.SNI = q.Get("sni")
	c.Fingerprint = q.Get("fp")
	if a := q.Get("alpn"); a != "" {
		c.ALPN = strings.Split(a, ",")
	}
	c.Insecure = q.Get("allowInsecure") == "1" || q.Get("insecure") == "1"
	c.PublicKey = q.Get("pbk")
	c.ShortID = q.Get("sid")
	if c.Security == "reality" && c.PublicKey == "" {
		return nil, fmt.Errorf("reality: missing pbk (public key)")
	}

	c.Transport = strings.ToLower(q.Get("type"))
	switch c.Transport {
	case "", "tcp", "raw":
		c.Transport = "tcp"
		if ht := q.Get("headerType"); ht != "" && ht != "none" {
			return nil, fmt.Errorf("tcp headerType %q not supported", ht)
		}
	case "ws", "grpc", "http", "h2", "httpupgrade":
		if c.Transport == "h2" {
			c.Transport = "http"
		}
	default:
		return nil, fmt.Errorf("transport %q not supported", c.Transport)
	}
	c.Path = q.Get("path")
	c.HostHeader = q.Get("host")
	c.ServiceName = q.Get("serviceName")

	if c.Transport == "ws" {
		if p, rawQ, ok := strings.Cut(c.Path, "?"); ok {
			if pq, err := url.ParseQuery(rawQ); err == nil {
				if ed, err := strconv.ParseUint(pq.Get("ed"), 10, 32); err == nil {
					c.EarlyData = uint32(ed)
					c.Path = p
				}
			}
		}
	}
	return c, nil
}

func (c *Config) Endpoint() (host, port string) {
	return c.Host, strconv.Itoa(int(c.Port))
}

func (c Config) String() string {
	return fmt.Sprintf("vless %s security=%s transport=%s",
		net.JoinHostPort(c.Host, strconv.Itoa(int(c.Port))), c.Security, c.Transport)
}
