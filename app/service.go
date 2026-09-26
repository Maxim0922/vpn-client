package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/max-tsx/max-vpn/internal/daemon"
	"github.com/max-tsx/max-vpn/internal/ipc"
	"github.com/max-tsx/max-vpn/internal/logbuf"
	"github.com/max-tsx/max-vpn/internal/stats"
)

type VPN struct {
	mu     sync.Mutex
	client *ipc.Client

	emit func(name string, data ...any)
}

func (v *VPN) ServiceName() string { return "VPN" }

func (v *VPN) ensure() (*ipc.Client, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.client != nil {
		return v.client, nil
	}
	c, err := ipc.Dial(ipc.SocketPath)
	if err != nil {
		return nil, err
	}

	_ = c.Subscribe(func(method string, params json.RawMessage) {
		if v.emit != nil {
			var payload any
			_ = json.Unmarshal(params, &payload)
			v.emit(method, payload)
		}
	})
	v.client = c
	return c, nil
}

func (v *VPN) drop() {
	v.mu.Lock()
	if v.client != nil {
		_ = v.client.Close()
		v.client = nil
	}
	v.mu.Unlock()
}

func (v *VPN) call(method string, params, out any) error {
	c, err := v.ensure()
	if err != nil {
		return fmt.Errorf("daemon offline: %w", err)
	}
	if err := c.Call(method, params, out); err != nil {
		var rpcErr *ipc.RPCError
		if !errors.As(err, &rpcErr) {
			v.drop()
		}
		return err
	}
	return nil
}

func (v *VPN) Status() (daemon.Status, error) {
	var s daemon.Status
	err := v.call("Status", nil, &s)
	return s, err
}

func (v *VPN) Connect(serverId string) (daemon.Status, error) {
	var s daemon.Status
	err := v.call("Connect", map[string]string{"serverId": serverId}, &s)
	return s, err
}

func (v *VPN) Disconnect() error { return v.call("Disconnect", nil, nil) }

func (v *VPN) ListServers() ([]daemon.ServerInfo, error) {
	out := []daemon.ServerInfo{}
	err := v.call("ListServers", nil, &out)
	if out == nil {
		out = []daemon.ServerInfo{}
	}
	return out, err
}

func (v *VPN) ImportConfig(name, confText string) error {
	return v.call("ImportConfig", map[string]string{"name": name, "confText": confText}, nil)
}

func (v *VPN) RemoveServer(id string) error {
	return v.call("RemoveServer", map[string]string{"id": id}, nil)
}

func (v *VPN) ToggleFavorite(id string) error {
	return v.call("ToggleFavorite", map[string]string{"id": id}, nil)
}

func (v *VPN) GetSettings() (daemon.Settings, error) {
	var s daemon.Settings
	err := v.call("GetSettings", nil, &s)
	return withSlices(s), err
}

func withSlices(s daemon.Settings) daemon.Settings {
	if s.CustomDNS == nil {
		s.CustomDNS = []string{}
	}
	if s.Favorites == nil {
		s.Favorites = []string{}
	}
	return s
}

func (v *VPN) SetSettings(s daemon.Settings) (daemon.Settings, error) {
	var out daemon.Settings
	err := v.call("SetSettings", s, &out)
	return withSlices(out), err
}

func (v *VPN) Stats() (stats.Snapshot, error) {
	var s stats.Snapshot
	err := v.call("Stats", nil, &s)
	return s, err
}

func (v *VPN) Logs(since string) ([]logbuf.Entry, error) {
	out := []logbuf.Entry{}
	err := v.call("Logs", map[string]string{"since": since}, &out)
	if out == nil {
		out = []logbuf.Entry{}
	}
	return out, err
}

func (v *VPN) Subscribe() error {
	_, err := v.ensure()
	return err
}

func (v *VPN) LaunchAtLogin(enable bool) error { return SetLaunchAtLogin(enable) }

func (v *VPN) retryConnect() {
	for {
		if _, err := v.ensure(); err == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
}
