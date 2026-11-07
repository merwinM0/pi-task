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
