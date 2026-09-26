package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	conn net.Conn
	enc  *json.Encoder
	sc   *bufio.Scanner

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan Response

	onEvent  atomic.Pointer[func(method string, params json.RawMessage)]
	readOnce sync.Once
	closed   atomic.Bool
}

func Dial(path string) (*Client, error) {
	nc, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w (is maxvpnd running?)", path, err)
	}
	sc := bufio.NewScanner(nc)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	c := &Client{conn: nc, enc: json.NewEncoder(nc), sc: sc, pending: map[int64]chan Response{}}
	c.readOnce.Do(func() { go c.readLoop() })
	return c, nil
}

func (c *Client) readLoop() {
	for c.sc.Scan() {
		line := c.sc.Bytes()

		var probe struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(line, &probe)

		if probe.ID == nil && probe.Method != "" {
			if h := c.onEvent.Load(); h != nil {
				(*h)(probe.Method, probe.Params)
			}
			continue
		}
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.ID == nil {
			continue
		}
		c.mu.Lock()
		ch := c.pending[*resp.ID]
		delete(c.pending, *resp.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- resp
		}
	}

	c.closed.Store(true)
	c.mu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
}

func (c *Client) Call(method string, params, out any) error {
	if c.closed.Load() {
		return fmt.Errorf("connection closed")
	}
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		raw = b
	}
	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan Response, 1)
	c.mu.Lock()
	c.pending[id] = ch
	err := c.enc.Encode(Request{JSONRPC: "2.0", ID: &id, Method: method, Params: raw})
	c.mu.Unlock()
	if err != nil {
		return err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return fmt.Errorf("no response (connection closed)")
		}
		if resp.Error != nil {
			return resp.Error
		}
		if out != nil && len(resp.Result) > 0 {
			return json.Unmarshal(resp.Result, out)
		}
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("call %s timed out", method)
	}
}

func (c *Client) Subscribe(onEvent func(method string, params json.RawMessage)) error {
	h := onEvent
	c.onEvent.Store(&h)
	return c.Call("Subscribe", nil, nil)
}

func (c *Client) Close() error { return c.conn.Close() }
