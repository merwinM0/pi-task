package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"pi-task/internal/task"
)

var columns = &runewidth.Condition{EastAsianWidth: false, StrictEmojiNeutral: true}

// Clean prevents persisted text, paths, errors, or filenames from injecting ANSI.
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069') {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(s, "�"))
}

type row struct {
	text, color string
	bold        bool
}

const (
	accent  = "167;223;219" // 碧青, transparent terminal surface
	muted   = "150;199;196"
	info    = "183;230;255"
	success = "184;239;207"
	warning = "255;225;168"
	danger  = "255;183;197"
)

func label(state task.State) (string, string, string) {
	switch state {
	case task.Pending:
		return "○", "待开始", muted
	case task.Running:
		return "▶", "进行中", info
	case task.Done, task.Completed:
		return "✓", "已完成", success
	case task.Blocked:
		return "!", "已阻塞", warning
	case task.Cancelled:
		return "–", "已取消", muted
	case task.Failed:
		return "✗", "失败", danger
	default:
		return "?", "未知", danger
	}
}

func (r row) render(width int, color bool) string {
	text := columns.Truncate(Clean(r.text), width, "…")
	if !color {
		return text
	}
	prefix := ""
	if r.bold {
		prefix += "\033[1m"
	}
	if r.color != "" {
		prefix += "\033[38;2;" + r.color + "m"
	}
	if prefix == "" {
		return text
	}
	return prefix + text + "\033[0m"
}

func wrap(text string, width int) []string {
	if width < 1 {
		return nil
	}
	return strings.Split(columns.Wrap(Clean(text), width), "\n")
}

func PlainList(runs []task.Run) string {
	if len(runs) == 0 {
		return "暂无任务记录。由 Pi 使用 pi-task start 提交工作后，在这里查看。"
	}
	var lines []string
	for _, run := range runs {
		symbol, state, _ := label(run.State)
		lines = append(lines, fmt.Sprintf("%s %s  %d/%d · %s\n  ID: %s\n  %s · %s",
			symbol, Clean(run.Title), run.DoneCount(), len(run.Tasks), state, run.ID,
			run.UpdatedAt.Local().Format("01-02 15:04:05"), Clean(run.Directory)))
	}
	return strings.Join(lines, "\n\n")
}

func PlainRun(run task.Run) string {
	symbol, state, _ := label(run.State)
	lines := []string{fmt.Sprintf("%s %s · %s · %d/%d", symbol, Clean(run.Title), state, run.DoneCount(), len(run.Tasks)),
		"ID: " + run.ID, "目录: " + Clean(run.Directory), "最近更新: " + run.UpdatedAt.Local().Format("2006-01-02 15:04:05")}
	for _, item := range run.Tasks {
		symbol, state, _ := label(item.State)
		lines = append(lines, fmt.Sprintf("%s %d. %s · %s", symbol, item.ID, state, Clean(item.Text)))
		if item.Note != "" {
			lines = append(lines, "    "+Clean(item.Note))
		}
	}
	return strings.Join(lines, "\n")
}
