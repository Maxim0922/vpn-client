package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/max-tsx/max-vpn/internal/state"
)

type Settings struct {
	KillSwitch      bool     `json:"killSwitch"`
	AllowLAN        bool     `json:"allowLAN"`
	CustomDNS       []string `json:"customDNS"`
	ConnectOnLaunch bool     `json:"connectOnLaunch"`

	LastServer string `json:"lastServer"`

	Favorites []string `json:"favorites"`

	PublicIPService string `json:"publicIPService"`
}

func settingsPath() string { return filepath.Join(state.DefaultDir, "settings.json") }

func LoadSettings() (*Settings, error) {
	s := &Settings{PublicIPService: "https://api.ipify.org"}
	data, err := os.ReadFile(settingsPath())
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.PublicIPService == "" {
		s.PublicIPService = "https://api.ipify.org"
	}
	return s, nil
}

func SaveSettings(s *Settings) error {
	if err := os.MkdirAll(state.DefaultDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := settingsPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, settingsPath())
}
