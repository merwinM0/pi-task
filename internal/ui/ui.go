// Package ui implements a read-only alternate-screen viewer. No model or network calls.
package ui

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"pi-task/internal/store"
)

func Run(s *store.Store, in, out *os.File, initialID string) (int, error) {
	original, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return 0, err
	}
	defer term.Restore(int(in.Fd()), original)
	defer fmt.Fprint(out, "\033[0m\033[?25h\033[?1049l")
	if _, err := fmt.Fprint(out, "\033[?1049h\033[?25l\033[2J"); err != nil {
		return 0, err
	}
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGWINCH)
	defer signal.Stop(signals)
	_, noColor := os.LookupEnv("NO_COLOR")
	v := view{detailID: initialID}
	v.refresh(s)
	var input decoder
	lastRefresh := time.Now()
	previous := ""
	buffer := make([]byte, 256)
	for {
		select {
		case sig := <-signals:
			switch sig {
			case syscall.SIGINT:
				return 130, nil
			case syscall.SIGTERM:
				return 143, nil
			case syscall.SIGHUP:
				return 129, nil
			case syscall.SIGWINCH:
				previous = ""
			}
		default:
		}
		if time.Since(lastRefresh) >= 500*time.Millisecond {
			v.refresh(s)
			lastRefresh = time.Now()
		}
		width, height, err := term.GetSize(int(out.Fd()))
		if err != nil {
			return 0, err
		}
		if width < 1 {
			width = 80
		}
		if height < 1 {
			height = 24
		}
		frame := v.frame(width, height, !noColor)
		if frame != previous {
			if _, err := fmt.Fprint(out, "\033[H", frame, "\033[J"); err != nil {
				return 0, err
			}
			previous = frame
		}
		fds := []unix.PollFd{{Fd: int32(in.Fd()), Events: unix.POLLIN}}
		_, err = unix.Poll(fds, 50)
		if err != nil && !errors.Is(err, syscall.EINTR) {
			return 0, err
		}
		var data []byte
		if fds[0].Revents&unix.POLLIN != 0 {
			n, e := in.Read(buffer)
			if e != nil {
				return 0, e
			}
			if n == 0 {
				return 0, nil
			}
			data = buffer[:n]
		} else if fds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return 0, nil
		}
		for _, k := range input.feed(data, time.Now()) {
			switch k {
			case quit:
				return 0, nil
			case interrupt:
				return 130, nil
			case back:
				if v.detailID == "" {
					return 0, nil
				}
				v.detailID, v.current, v.offset = "", nil, 0
			case enter:
				if v.detailID == "" && v.selected != "" {
					v.detailID, v.offset = v.selected, 0
					v.refresh(s)
				}
			case up:
				v.move(-1)
			case down:
				v.move(1)
			case pageUp:
				v.move(-max(1, height/2))
			case pageDown:
				v.move(max(1, height/2))
			case home:
				v.move(-1000000)
			case end:
				v.move(1000000)
			}
		}
	}
}
