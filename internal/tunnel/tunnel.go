package tunnel

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

type Tunnel struct {
	dev  *device.Device
	tun  tun.Device
	name string
}

func Create(name string, mtu int, logger *device.Logger) (*Tunnel, error) {
	tunDev, err := tun.CreateTUN(name, mtu)
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}
	real, err := tunDev.Name()
	if err != nil {
		_ = tunDev.Close()
		return nil, fmt.Errorf("get tun name: %w", err)
	}
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)
	return &Tunnel{dev: dev, tun: tunDev, name: real}, nil
}

func (t *Tunnel) Name() string { return t.name }

func (t *Tunnel) Configure(uapi string) error {
	if err := t.dev.IpcSet(uapi); err != nil {
		return fmt.Errorf("ipc set: %w", err)
	}
	return nil
}

func (t *Tunnel) Up() error {
	if err := t.dev.Up(); err != nil {
		return fmt.Errorf("device up: %w", err)
	}
	return nil
}

type Stats struct {
	RxBytes       uint64
	TxBytes       uint64
	LastHandshake time.Time
}

func (t *Tunnel) Stats() (Stats, error) {
	uapi, err := t.dev.IpcGet()
	if err != nil {
		return Stats{}, fmt.Errorf("ipc get: %w", err)
	}
	return parseStats(uapi), nil
}

func parseStats(uapi string) Stats {
	var s Stats
	sc := bufio.NewScanner(strings.NewReader(uapi))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		switch k {
		case "rx_bytes":
			s.RxBytes, _ = strconv.ParseUint(v, 10, 64)
		case "tx_bytes":
			s.TxBytes, _ = strconv.ParseUint(v, 10, 64)
		case "last_handshake_time_sec":
			sec, _ := strconv.ParseInt(v, 10, 64)
			if sec > 0 {
				s.LastHandshake = time.Unix(sec, 0)
			}
		}
	}
	return s
}

func (t *Tunnel) HasHandshake() bool {
	s, err := t.Stats()
	return err == nil && !s.LastHandshake.IsZero()
}

func (t *Tunnel) Close() error {
	if t.dev != nil {
		t.dev.Close()
	}
	return nil
}
