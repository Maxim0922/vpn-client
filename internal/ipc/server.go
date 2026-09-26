package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"sync"
)

type Handler func(ctx context.Context, params json.RawMessage) (any, error)

type conn struct {
	enc *json.Encoder
	mu  sync.Mutex
	sub bool
}

func (c *conn) write(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enc.Encode(v)
}

type Server struct {
	ln       net.Listener
	handlers map[string]Handler
	log      func(string)

	mu   sync.Mutex
	subs map[*conn]struct{}
}

func Listen(path string, log func(string)) (*Server, error) {
	if log == nil {
		log = func(string) {}
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o660); err != nil {
		_ = ln.Close()
		return nil, err
	}
	if g, err := user.LookupGroup(SocketGroup); err == nil {
		if gid, err := strconv.Atoi(g.Gid); err == nil {
			_ = os.Chown(path, -1, gid)
		}
	}
	return &Server{ln: ln, handlers: map[string]Handler{}, log: log, subs: map[*conn]struct{}{}}, nil
}

func (s *Server) Handle(method string, h Handler) { s.handlers[method] = h }

func (s *Server) Broadcast(method string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	note := Notification{JSONRPC: "2.0", Method: method, Params: raw}
	s.mu.Lock()
	targets := make([]*conn, 0, len(s.subs))
	for c := range s.subs {
		targets = append(targets, c)
	}
	s.mu.Unlock()
	for _, c := range targets {
		if err := c.write(note); err != nil {
			s.removeSub(c)
		}
	}
}

func (s *Server) addSub(c *conn)    { s.mu.Lock(); s.subs[c] = struct{}{}; s.mu.Unlock() }
func (s *Server) removeSub(c *conn) { s.mu.Lock(); delete(s.subs, c); s.mu.Unlock() }

func (s *Server) Serve(ctx context.Context) error {
	go func() { <-ctx.Done(); _ = s.ln.Close() }()
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.handleConn(ctx, nc)
	}
}

func (s *Server) Close() error {
	err := s.ln.Close()
	if ua, ok := s.ln.Addr().(*net.UnixAddr); ok {
		_ = os.Remove(ua.Name)
	}
	return err
}

func (s *Server) handleConn(ctx context.Context, nc net.Conn) {
	defer nc.Close()

	if err := authorize(nc); err != nil {
		s.log("rejected connection: " + err.Error())
		enc := json.NewEncoder(nc)
		_ = enc.Encode(Response{JSONRPC: "2.0", Error: &RPCError{Code: CodeUnauthorized, Message: "unauthorized"}})
		return
	}

	c := &conn{enc: json.NewEncoder(nc)}
	defer s.removeSub(c)

	sc := bufio.NewScanner(nc)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var req Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			_ = c.write(Response{JSONRPC: "2.0", Error: &RPCError{Code: CodeParse, Message: err.Error()}})
			continue
		}

		if req.Method == "Subscribe" {
			s.addSub(c)
			c.sub = true
			_ = c.write(Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"subscribed":true}`)})
			continue
		}

		h, ok := s.handlers[req.Method]
		if !ok {
			_ = c.write(Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeMethodNotFn, Message: "unknown method: " + req.Method}})
			continue
		}
		result, err := h(ctx, req.Params)
		if err != nil {
			_ = c.write(Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternal, Message: err.Error()}})
			continue
		}
		raw, err := json.Marshal(result)
		if err != nil {
			_ = c.write(Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternal, Message: "marshal result: " + err.Error()}})
			continue
		}
		_ = c.write(Response{JSONRPC: "2.0", ID: req.ID, Result: raw})
	}
}
