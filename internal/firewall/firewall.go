package firewall

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
)

const Anchor = "com.apple/maxvpn"

type Rules struct {
	Iface         string
	EndpointIP    string
	EndpointPort  string
	EndpointProto string
	AllowLAN      bool
	HasV6         bool
}

var lanV4 = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"169.254.0.0/16",
	"224.0.0.0/4",
}

var lanV6 = []string{
	"fe80::/10",
	"fc00::/7",
	"ff00::/8",
}

func (r Rules) Generate() string {
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }

	w("# max-vpn kill switch (anchor %s)", Anchor)
	w("set block-policy drop")
	w("block all")
	w("pass quick on lo0 all")

	if r.Iface != "" {
		w("pass quick on %s all", r.Iface)
	}

	if r.EndpointIP != "" && r.EndpointPort != "" {
		proto := r.EndpointProto
		if proto == "" {
			proto = "udp"
		}
		w("pass out quick proto %s to %s port %s", proto, r.EndpointIP, r.EndpointPort)
	}

	w("pass out quick proto udp from any port 68 to any port 67")
	w("pass in quick proto udp from any port 67 to any port 68")

	if r.AllowLAN {
		w("pass quick inet from any to { %s }", strings.Join(lanV4, ", "))
		w("pass quick inet from { %s } to any", strings.Join(lanV4, ", "))
		w("pass quick inet6 from any to { %s }", strings.Join(lanV6, ", "))
		w("pass quick inet6 from { %s } to any", strings.Join(lanV6, ", "))
	}

	return b.String()
}

var tokenRe = regexp.MustCompile(`Token\s*:\s*(\d+)`)

func Enable(ctx context.Context, run xexec.Runner, rules Rules) (token string, err error) {
	ruleset := rules.Generate()
	if _, err := run.RunInput(ctx, []byte(ruleset), "pfctl", "-a", Anchor, "-f", "-"); err != nil {
		return "", fmt.Errorf("load anchor rules: %w", err)
	}
	out, err := run.Run(ctx, "pfctl", "-E")
	if err != nil {
		return "", fmt.Errorf("enable pf: %w", err)
	}
	m := tokenRe.FindSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("no pf token in output: %q", string(out))
	}
	return string(m[1]), nil
}

func Disable(ctx context.Context, run xexec.Runner, token string) error {
	var firstErr error
	if token != "" {
		if _, err := run.Run(ctx, "pfctl", "-X", token); err != nil {
			firstErr = fmt.Errorf("release pf token: %w", err)
		}
	}
	if _, err := run.Run(ctx, "pfctl", "-a", Anchor, "-F", "all"); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("flush anchor: %w", err)
	}
	return firstErr
}
