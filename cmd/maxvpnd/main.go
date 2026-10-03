package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/max-tsx/max-vpn/internal/daemon"
	xexec "github.com/max-tsx/max-vpn/internal/exec"
	"github.com/max-tsx/max-vpn/internal/ipc"
	"github.com/max-tsx/max-vpn/internal/monitor"
	"github.com/max-tsx/max-vpn/internal/openvpn"
	"github.com/max-tsx/max-vpn/internal/vless"
	"github.com/max-tsx/max-vpn/internal/vless/singbox"
)

func init() {
	daemon.StartVLESS = func(c *vless.Config, o vless.Options) (daemon.VLESSTunnel, error) {
		t, err := singbox.Start(c, o)
		if err != nil {
			return nil, err
		}
		return t, nil
	}
	daemon.StartOpenVPN = func(ctx context.Context, c *openvpn.Config, confPath string, iface string, log func(string)) (daemon.OpenVPNTunnel, error) {
		t, err := openvpn.Start(ctx, c, confPath, iface, log)
		if err != nil {
			return nil, err
		}
		return t, nil
	}
}

func main() {
	logger := log.New(os.Stderr, "maxvpnd ", log.LstdFlags)
	logf := func(s string) { logger.Println(s) }

	var runner xexec.Runner = xexec.RealRunner{}
	if os.Getenv("MAXVPN_DRYRUN") == "1" {
		dr := xexec.NewDryRunner()
		dr.Print = true
		dr.Printer = logf
		runner = dr
		logf("DRY-RUN mode: system commands will be printed, not executed")
	}

	mgr := daemon.NewManager(runner, logf)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := mgr.RecoverFromCrash(ctx); err != nil {
		logf("crash recovery: " + err.Error())
	}

	srv, err := ipc.Listen(ipc.SocketPath, logf)
	if err != nil {
		logger.Fatal(err)
	}
	registerHandlers(srv, mgr)

	logf("listening on " + ipc.SocketPath)

	go broadcaster(ctx, srv, mgr)

	go func() {
		if err := monitor.Run(ctx, func() { mgr.OnNetworkChange(ctx) }, logf); err != nil {
			logf("route monitor: " + err.Error())
		}
	}()

	if set := mgr.GetSettings(); set.ConnectOnLaunch && set.LastServer != "" {
		go func() {
			logf("connect-on-launch: " + set.LastServer)
			if err := mgr.Connect(ctx, set.LastServer); err != nil {
				logf("connect-on-launch failed: " + err.Error())
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx) }()

	select {
	case <-ctx.Done():
		logf("signal received, tearing down")
	case err := <-errCh:
		if err != nil {
			logf("serve error: " + err.Error())
		}
	}

	if err := mgr.Disconnect(context.Background()); err != nil {
		logf("shutdown teardown: " + err.Error())
	}
	_ = srv.Close()
	logf("stopped")
}

func broadcaster(ctx context.Context, srv *ipc.Server, mgr *daemon.Manager) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	var lastState string
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			st := mgr.Status()

			if st.State != lastState {
				srv.Broadcast("status", st)
				lastState = st.State
			} else if st.State == daemon.StateConnected {
				srv.Broadcast("status", st)
			}
			if st.State == daemon.StateConnected {
				srv.Broadcast("stats", mgr.Stats(ctx))
			}
		}
	}
}

func registerHandlers(srv *ipc.Server, mgr *daemon.Manager) {
	srv.Handle("Status", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return mgr.Status(), nil
	})
	srv.Handle("Connect", func(ctx context.Context, p json.RawMessage) (any, error) {
		var args struct {
			ServerId string `json:"serverId"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		if args.ServerId == "" {
			return nil, fmt.Errorf("serverId required")
		}
		if err := mgr.Connect(ctx, args.ServerId); err != nil {
			return nil, err
		}
		return mgr.Status(), nil
	})
	srv.Handle("Disconnect", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return nil, mgr.Disconnect(ctx)
	})
	srv.Handle("ListServers", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return mgr.ListServers()
	})
	srv.Handle("ImportConfig", func(ctx context.Context, p json.RawMessage) (any, error) {
		var args struct {
			Name     string `json:"name"`
			ConfText string `json:"confText"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		if args.Name == "" {
			return nil, fmt.Errorf("name required")
		}
		return nil, mgr.ImportConfig(args.Name, args.ConfText)
	})
	srv.Handle("RemoveServer", func(ctx context.Context, p json.RawMessage) (any, error) {
		var args struct {
			Id string `json:"id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return nil, mgr.RemoveServer(args.Id)
	})
	srv.Handle("ToggleFavorite", func(ctx context.Context, p json.RawMessage) (any, error) {
		var args struct {
			Id string `json:"id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return nil, mgr.ToggleFavorite(args.Id)
	})
	srv.Handle("GetSettings", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return mgr.GetSettings(), nil
	})
	srv.Handle("SetSettings", func(ctx context.Context, p json.RawMessage) (any, error) {
		var s daemon.Settings
		if err := json.Unmarshal(p, &s); err != nil {
			return nil, err
		}
		if err := mgr.SetSettings(s); err != nil {
			return nil, err
		}
		return mgr.GetSettings(), nil
	})
	srv.Handle("Stats", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return mgr.Stats(ctx), nil
	})
	srv.Handle("Logs", func(ctx context.Context, p json.RawMessage) (any, error) {
		var args struct {
			Since string `json:"since"`
		}
		_ = json.Unmarshal(p, &args)
		since := time.Time{}
		if args.Since != "" {
			if t, err := time.Parse(time.RFC3339, args.Since); err == nil {
				since = t
			}
		}
		return mgr.Logs(since), nil
	})
}
