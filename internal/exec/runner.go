package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)

	RunInput(ctx context.Context, in []byte, name string, args ...string) ([]byte, error)
}

type RealRunner struct{}

func (RealRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return RealRunner{}.RunInput(ctx, nil, name, args...)
}

func (RealRunner) RunInput(ctx context.Context, in []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if in != nil {
		cmd.Stdin = bytes.NewReader(in)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("exec %s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

type DryRunner struct {
	mu      sync.Mutex
	Cmds    [][]string
	Stdin   [][]byte
	Print   bool
	Printer func(string)

	Outputs map[string][][]byte

	Errors map[string][]error
}

func NewDryRunner() *DryRunner {
	return &DryRunner{Outputs: map[string][][]byte{}, Errors: map[string][]error{}}
}

func (d *DryRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return d.RunInput(ctx, nil, name, args...)
}

func (d *DryRunner) RunInput(_ context.Context, in []byte, name string, args ...string) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	line := append([]string{name}, args...)
	d.Cmds = append(d.Cmds, line)
	d.Stdin = append(d.Stdin, in)
	if d.Print {
		msg := "would run: " + strings.Join(line, " ")
		if d.Printer != nil {
			d.Printer(msg)
		} else {
			fmt.Println(msg)
		}
	}

	var out []byte
	if q := d.Outputs[name]; len(q) > 0 {
		out = q[0]
		d.Outputs[name] = q[1:]
	}
	var err error
	if q := d.Errors[name]; len(q) > 0 {
		err = q[0]
		d.Errors[name] = q[1:]
	}
	return out, err
}

func (d *DryRunner) Lines() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	res := make([]string, len(d.Cmds))
	for i, c := range d.Cmds {
		res[i] = strings.Join(c, " ")
	}
	return res
}
