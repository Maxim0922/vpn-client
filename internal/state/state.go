package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
)

var DefaultDir = "/Library/Application Support/max-vpn"

const fileName = "state.json"

type State struct {
	Iface string `json:"iface"`

	EndpointIP string `json:"endpoint_ip"`

	EndpointRouteInstalled bool `json:"endpoint_route_installed"`

	OrigGWv4 string `json:"orig_gw_v4"`
	OrigGWv6 string `json:"orig_gw_v6"`

	OrigDNS map[string][]string `json:"orig_dns"`

	DNSApplied []string `json:"dns_applied"`

	PFToken string `json:"pf_token"`

	IPv6Blocked bool `json:"ipv6_blocked"`
}

func path() string { return filepath.Join(DefaultDir, fileName) }

func Save(s *State) error {
	if err := os.MkdirAll(DefaultDir, 0o700); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return os.Rename(tmp, path())
}

func Load() (*State, error) {
	data, err := os.ReadFile(path())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &s, nil
}

func Clear() error {
	err := os.Remove(path())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func Teardown(ctx context.Context, r xexec.Runner, s *State, log func(string)) error {
	if s == nil {
		return nil
	}
	logf := func(f string, a ...any) {
		if log != nil {
			log(fmt.Sprintf(f, a...))
		}
	}
	var errs []error
	try := func(step string, err error) {
		if err != nil {
			logf("teardown: %s failed: %v", step, err)
			errs = append(errs, fmt.Errorf("%s: %w", step, err))
		}
	}

	if s.PFToken != "" {
		_, err := r.Run(ctx, "pfctl", "-X", s.PFToken)
		try("pf disable", err)
		_, err = r.Run(ctx, "pfctl", "-a", "com.apple/maxvpn", "-F", "all")
		try("pf flush anchor", err)
	}

	if s.EndpointRouteInstalled && s.EndpointIP != "" {
		_, err := r.Run(ctx, "route", "-n", "delete", "-host", s.EndpointIP)
		try("delete endpoint route", err)
	}

	if s.IPv6Blocked {
		_, err := r.Run(ctx, "route", "-n", "delete", "-inet6", "::/1")
		try("unblock ::/1", err)
		_, err = r.Run(ctx, "route", "-n", "delete", "-inet6", "8000::/1")
		try("unblock 8000::/1", err)
	}

	for _, svc := range s.DNSApplied {
		orig := s.OrigDNS[svc]
		args := []string{"-setdnsservers", svc}
		if len(orig) == 0 {
			args = append(args, "Empty")
		} else {
			args = append(args, orig...)
		}
		_, err := r.Run(ctx, "networksetup", args...)
		try("restore dns "+svc, err)
	}

	if s.Iface != "" {
		_, _ = r.Run(ctx, "ifconfig", s.Iface, "destroy")
	}

	if err := Clear(); err != nil {
		try("clear state file", err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("teardown had %d error(s): %v", len(errs), errs)
	}
	return nil
}
