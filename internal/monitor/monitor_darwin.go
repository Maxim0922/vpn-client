package monitor

import (
	"context"
	"time"

	"golang.org/x/sys/unix"
)

const debounce = 1500 * time.Millisecond

func Run(ctx context.Context, onChange func(), logf func(string)) error {
	if logf == nil {
		logf = func(string) {}
	}
	fd, err := unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, unix.AF_UNSPEC)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_ = unix.Close(fd)
	}()

	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				onChange()
			}
		}
	}()

	buf := make([]byte, 4096)
	for {
		n, err := unix.Read(fd, buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if n < 4 {
			continue
		}

		if interesting(buf[3]) {
			timer.Reset(debounce)
		}
	}
}

func interesting(msgType byte) bool {
	switch msgType {
	case unix.RTM_NEWADDR, unix.RTM_DELADDR, unix.RTM_IFINFO,
		unix.RTM_ADD, unix.RTM_DELETE, unix.RTM_REDIRECT:
		return true
	}
	return false
}
