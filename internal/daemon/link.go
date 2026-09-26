package daemon

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
	"github.com/max-tsx/max-vpn/internal/tunnel"
	"github.com/max-tsx/max-vpn/internal/vless"
)

type link interface {
	Name() string
	Stats() (tunnel.Stats, error)
	Close() error
}

type VLESSTunnel interface {
	Name() string
	Probe(ctx context.Context) error
	LastAlive() time.Time
	Close() error
}

var StartVLESS func(c *vless.Config, o vless.Options) (VLESSTunnel, error)

type vlessLink struct {
	VLESSTunnel
	runner xexec.Runner
}

func (l vlessLink) Stats() (tunnel.Stats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := l.runner.Run(ctx, "netstat", "-I", l.Name(), "-b", "-n")
	if err != nil {
		return tunnel.Stats{}, err
	}
	rx, tx, err := parseNetstatBytes(out, l.Name())
	if err != nil {
		return tunnel.Stats{}, err
	}
	return tunnel.Stats{RxBytes: rx, TxBytes: tx, LastHandshake: l.LastAlive()}, nil
}

func parseNetstatBytes(out []byte, iface string) (rx, tx uint64, err error) {
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 9 && f[0] == iface && strings.HasPrefix(f[2], "<Link") {
			rx, err1 := strconv.ParseUint(f[5], 10, 64)
			tx, err2 := strconv.ParseUint(f[8], 10, 64)
			if err1 != nil || err2 != nil {
				return 0, 0, fmt.Errorf("netstat: bad counters in %q", sc.Text())
			}
			return rx, tx, nil
		}
	}
	return 0, 0, fmt.Errorf("netstat: no link row for %s", iface)
}

func nextUtun() (string, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	next := 0
	for _, i := range ifs {
		if n, err := strconv.Atoi(strings.TrimPrefix(i.Name, "utun")); err == nil && strings.HasPrefix(i.Name, "utun") && n >= next {
			next = n + 1
		}
	}
	return fmt.Sprintf("utun%d", next), nil
}
