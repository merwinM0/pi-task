package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"pi-task/internal/store"
	"pi-task/internal/task"
)

func TestKeysAndSplitEscapeSequences(t *testing.T) {
	d := decoder{}
	now := time.Now()
	if keys := d.feed([]byte("wS\r"), now); fmt.Sprint(keys) != fmt.Sprint([]key{up, down, enter}) {
		t.Fatal(keys)
	}
	if keys := d.feed([]byte("\033["), now); len(keys) != 0 {
		t.Fatal(keys)
	}
	if keys := d.feed([]byte("B"), now.Add(time.Millisecond)); len(keys) != 1 || keys[0] != down {
		t.Fatal(keys)
	}
	if keys := d.feed([]byte("\033"), now); len(keys) != 0 {
		t.Fatal(keys)
	}
	if keys := d.feed(nil, now.Add(100*time.Millisecond)); len(keys) != 1 || keys[0] != back {
		t.Fatal(keys)
	}
	if keys := d.feed([]byte("\003"), now); len(keys) != 1 || keys[0] != interrupt {
		t.Fatal(keys)
	}
}

func TestPagingKeysAndSplitLeftRightSequences(t *testing.T) {
	for _, test := range []struct {
		input string
		want  key
	}{
		{"a", pageUp}, {"A", pageUp}, {"d", pageDown}, {"D", pageDown},
		{"\033[D", pageUp}, {"\033[C", pageDown}, {"\033OD", pageUp}, {"\033OC", pageDown},
		{"\033[5~", pageUp}, {"\033[6~", pageDown},
	} {
		t.Run(fmt.Sprintf("%q", test.input), func(t *testing.T) {
			d := decoder{}
			keys := d.feed([]byte(test.input), time.Now())
			if len(keys) != 1 || keys[0] != test.want {
				t.Fatal(keys)
			}
		})
	}
	for _, suffix := range []byte{'C', 'D'} {
		d := decoder{}
		now := time.Now()
		if keys := d.feed([]byte("\033["), now); len(keys) != 0 {
			t.Fatal(keys)
		}
		keys := d.feed([]byte{suffix}, now.Add(time.Millisecond))
		want := pageDown
		if suffix == 'D' {
			want = pageUp
		}
		if len(keys) != 1 || keys[0] != want {
			t.Fatal(keys)
		}
	}
}

func TestPagingHelpVisibleInWideAndNarrowViews(t *testing.T) {
	for _, detail := range []string{"", "test"} {
		for _, width := range []int{20, 40, 80} {
			v := view{detailID: detail}
			frame := v.frame(width, 24, false)
			if !strings.Contains(frame, "A/D") && !strings.Contains(frame, "AD") {
				t.Fatal(width, frame)
			}
		}
	}
}

func TestFrameBoundsChineseEmojiAndSmallTerminals(t *testing.T) {
	now := time.Now()
	r := task.Run{ID: "test", Title: "中文标题 👩‍💻 very long title", Directory: "/home/mo/项目", State: task.Running, UpdatedAt: now,
		Tasks: []task.Item{{ID: 1, Text: strings.Repeat("检查中文🚀", 20), State: task.Running, Note: strings.Repeat("测试依据", 30)}}}
	v := view{runs: []task.Run{r}, current: &r, detailID: "test", now: now}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	for _, size := range [][2]int{{80, 24}, {40, 12}, {20, 8}, {8, 4}, {140, 40}} {
		for _, color := range []bool{true, false} {
			frame := v.frame(size[0], size[1], color)
			lines := strings.Split(ansi.ReplaceAllString(frame, ""), "\r\n")
			if len(lines) > max(1, size[1]-1) {
				t.Fatal("too tall", size, len(lines))
			}
			for _, line := range lines {
				if columns.StringWidth(line) > size[0]-1 {
					t.Fatalf("too wide %v: %q", size, line)
				}
			}
		}
	}
}

func TestSelectionSurvivesReorderAndLiveUpdates(t *testing.T) {
	s := store.Store{Root: t.TempDir()}
	_, _ = s.Create("first", "first", "/", []string{"a"})
	_, _ = s.Create("second", "second", "/", []string{"b"})
	v := view{}
	v.refresh(&s)
	v.move(1)
	if v.selected != "first" {
		t.Fatal(v.selected)
	}
	_, _ = s.Change("first", func(r *task.Run) error { return r.Update(1, task.Done, nil, nil, 0) })
	v.refresh(&s)
	if v.selected != "first" || v.index != 0 {
		t.Fatal(v.selected, v.index)
	}
	v.detailID = "first"
	v.refresh(&s)
	if v.current.DoneCount() != 1 {
		t.Fatal("detail was not refreshed")
	}
}

func TestControlCharactersAndReadOnlyErrorsAreVisible(t *testing.T) {
	clean := Clean("danger\033[31m\r\n\u202e")
	if strings.ContainsAny(clean, "\033\r\n\u202e") {
		t.Fatal(clean)
	}
	v := view{readError: "读取失败，以下可能是旧快照", now: time.Now()}
	if frame := v.frame(80, 24, false); !strings.Contains(frame, "读取失败") {
		t.Fatal(frame)
	}
}
