package netcfg

import (
	"bufio"
	"context"
	"fmt"
	"net/netip"
	"strings"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
)

func AddressPlan(iface string, addrs []netip.Prefix) [][]string {
	var plan [][]string
	for _, p := range addrs {
		a := p.Addr()
		if a.Is4() {
			plan = append(plan, []string{"ifconfig", iface, "inet", a.String(), a.String(), "alias"})
		} else {
			plan = append(plan, []string{"ifconfig", iface, "inet6", a.String(), "prefixlen", fmt.Sprint(p.Bits()), "alias"})
		}
	}
	plan = append(plan, []string{"ifconfig", iface, "up"})
	return plan
}

func EndpointRoutePlan(endpointIP, gateway string) [][]string {
	return [][]string{{"route", "-n", "add", "-host", endpointIP, gateway}}
}

func FullTunnelPlan(iface string, hasV6 bool) (plan [][]string, ipv6Blocked bool) {
	plan = append(plan,
		[]string{"route", "-n", "add", "-inet", "0.0.0.0/1", "-interface", iface},
		[]string{"route", "-n", "add", "-inet", "128.0.0.0/1", "-interface", iface},
	)
	if hasV6 {
		plan = append(plan,
			[]string{"route", "-n", "add", "-inet6", "::/1", "-interface", iface},
			[]string{"route", "-n", "add", "-inet6", "8000::/1", "-interface", iface},
		)
		return plan, false
	}
	plan = append(plan,
		[]string{"route", "-n", "add", "-inet6", "::/1", "::1", "-blackhole"},
		[]string{"route", "-n", "add", "-inet6", "8000::/1", "::1", "-blackhole"},
	)
	return plan, true
}

func DNSPlan(services []string, dns []netip.Addr) [][]string {
	strs := make([]string, len(dns))
	for i, d := range dns {
		strs[i] = d.String()
	}
	var plan [][]string
	for _, svc := range services {
		args := append([]string{"-setdnsservers", svc}, strs...)
		plan = append(plan, append([]string{"networksetup"}, args...))
	}
	return plan
}

func Apply(ctx context.Context, r xexec.Runner, plan [][]string) error {
	for _, cmd := range plan {
		if len(cmd) == 0 {
			continue
		}
		if _, err := r.Run(ctx, cmd[0], cmd[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func DefaultGatewayV4(ctx context.Context, r xexec.Runner) (string, error) {
	out, err := r.Run(ctx, "route", "-n", "get", "-inet", "default")
	if err != nil {
		return "", err
	}
	return parseGateway(string(out))
}

func DefaultGatewayV6(ctx context.Context, r xexec.Runner) (string, error) {
	out, err := r.Run(ctx, "route", "-n", "get", "-inet6", "default")
	if err != nil {
		return "", nil
	}
	gw, _ := parseGateway(string(out))
	return gw, nil
}

func parseGateway(out string) (string, error) {
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if v, ok := strings.CutPrefix(line, "gateway:"); ok {
			return strings.TrimSpace(v), nil
		}
	}
	return "", fmt.Errorf("no gateway in route output")
}

func ActiveServices(ctx context.Context, r xexec.Runner) ([]string, error) {
	out, err := r.Run(ctx, "networksetup", "-listallnetworkservices")
	if err != nil {
		return nil, err
	}
	var svcs []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	first := true
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if first {
			first = false
			continue
		}
		if line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		svcs = append(svcs, line)
	}
	return svcs, nil
}

func CurrentDNS(ctx context.Context, r xexec.Runner, services []string) (map[string][]string, error) {
	res := make(map[string][]string, len(services))
	for _, svc := range services {
		out, err := r.Run(ctx, "networksetup", "-getdnsservers", svc)
		if err != nil {
			return nil, err
		}
		res[svc] = parseDNS(string(out))
	}
	return res, nil
}

func parseDNS(out string) []string {
	var servers []string
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "There aren't any") {
			continue
		}
		servers = append(servers, line)
	}
	return servers
}
