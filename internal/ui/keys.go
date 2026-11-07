package ui

import (
	"bytes"
	"time"
)

type key int

const (
	up key = iota + 1
	down
	enter
	back
	quit
	interrupt
	pageUp
	pageDown
	home
	end
)

type decoder struct {
	pending  []byte
	escapeAt time.Time
}

func (d *decoder) feed(data []byte, now time.Time) []key {
	d.pending = append(d.pending, data...)
	var keys []key
	for len(d.pending) > 0 {
		c := d.pending[0]
		if c != 27 {
			d.pending = d.pending[1:]
			switch c {
			case 'w', 'W', 'k':
				keys = append(keys, up)
			case 's', 'S', 'j':
				keys = append(keys, down)
			case 'a', 'A':
				keys = append(keys, pageUp)
			case 'd', 'D':
				keys = append(keys, pageDown)
			case '\r', '\n':
				keys = append(keys, enter)
			case 'q', 'Q':
				keys = append(keys, quit)
			case 3:
				keys = append(keys, interrupt)
			}
			continue
		}
		if d.escapeAt.IsZero() {
			d.escapeAt = now
		}
		sequences := []struct {
			bytes string
			key   key
		}{
			{"\033[A", up}, {"\033[B", down}, {"\033OA", up}, {"\033OB", down},
			{"\033[D", pageUp}, {"\033[C", pageDown}, {"\033OD", pageUp}, {"\033OC", pageDown},
			{"\033[5~", pageUp}, {"\033[6~", pageDown}, {"\033[H", home}, {"\033[F", end},
			{"\033[1~", home}, {"\033[4~", end},
		}
		matched, prefix := false, false
		for _, seq := range sequences {
			if bytes.HasPrefix(d.pending, []byte(seq.bytes)) {
				keys = append(keys, seq.key)
				d.pending = d.pending[len(seq.bytes):]
				matched = true
				break
			}
			if bytes.HasPrefix([]byte(seq.bytes), d.pending) {
				prefix = true
			}
		}
		if matched {
			d.escapeAt = time.Time{}
			continue
		}
		if prefix && now.Sub(d.escapeAt) < 60*time.Millisecond {
			break
		}
		// Unknown complete CSI sequences are discarded, never treated as W/S keys.
		if len(d.pending) >= 2 && d.pending[1] == '[' {
			finish := -1
			for i := 2; i < len(d.pending); i++ {
				if d.pending[i] >= 0x40 && d.pending[i] <= 0x7e {
					finish = i
					break
				}
			}
			if finish < 0 && now.Sub(d.escapeAt) < 60*time.Millisecond {
				break
			}
			if finish >= 0 {
				d.pending = d.pending[finish+1:]
			} else {
				d.pending = nil
			}
		} else {
			keys = append(keys, back)
			d.pending = d.pending[1:]
		}
		d.escapeAt = time.Time{}
	}
	return keys
}
