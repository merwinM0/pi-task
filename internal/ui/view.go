package ui

import (
	"fmt"
	"strings"
	"time"

	"pi-task/internal/store"
	"pi-task/internal/task"
)

type view struct {
	runs      []task.Run
	selected  string
	index     int
	detailID  string
	current   *task.Run
	offset    int
	warning   string
	readError string
	now       time.Time
}

func (v *view) refresh(s *store.Store) {
	v.now = time.Now()
	list, err := s.Recent(5)
	if err != nil {
		v.readError = "读取失败，以下可能是旧快照：" + err.Error()
		return
	}
	v.runs = list.Runs
	v.readError, v.warning = "", ""
	if len(list.Warnings) > 0 {
		v.warning = fmt.Sprintf("%d 份损坏记录已跳过：%s", len(list.Warnings), list.Warnings[0])
	}
	found := false
	for i, run := range v.runs {
		if run.ID == v.selected {
			v.index, found = i, true
			break
		}
	}
	if !found && len(v.runs) > 0 {
		v.index = min(v.index, len(v.runs)-1)
		v.selected = v.runs[v.index].ID
	}
	if v.detailID != "" {
		run, err := s.Read(v.detailID)
		if err != nil {
			v.readError = "读取失败，以下可能是旧快照：" + err.Error()
		} else {
			v.current = &run
		}
	}
}

func (v *view) move(delta int) {
	if v.detailID != "" {
		v.offset = max(0, v.offset+delta)
		return
	}
	if len(v.runs) == 0 {
		return
	}
	v.index = max(0, min(len(v.runs)-1, v.index+delta))
	v.selected = v.runs[v.index].ID
}

func (v *view) body(width int) []row {
	if v.current == nil {
		return []row{{text: "正在等待任务记录…", color: muted}}
	}
	run := v.current
	if len(run.Tasks) == 0 {
		return []row{{text: "尚未提交步骤清单", color: muted}}
	}
	var body []row
	for _, item := range run.Tasks {
		symbol, state, color := label(item.State)
		text := fmt.Sprintf("%s %d. %s · %s", symbol, item.ID, state, item.Text)
		for _, part := range wrap(text, width) {
			body = append(body, row{text: part, color: color})
		}
		if item.Note != "" {
			for _, part := range wrap("备注："+item.Note, max(1, width-2)) {
				body = append(body, row{text: "  " + part, color: muted})
			}
		}
		body = append(body, row{})
	}
	return body
}

func (v *view) frame(width, height int, color bool) string {
	width = max(1, width-1) // Avoid a right-edge wrap, including after terminal resize.
	limit := max(1, height-1)
	if width < 16 || height < 6 {
		return row{text: "终端过小 · Q 退出", color: warning}.render(width, color)
	}
	var rows []row
	footer := "W/S ↑↓ 选择 · A/D ←→ 翻页 · Enter 查看 · Q 退出"
	if v.detailID == "" {
		rows = append(rows, row{text: "PI TASK  /  最近 5 次工作", color: accent, bold: true}, row{text: strings.Repeat("━", width), color: accent})
		if len(v.runs) == 0 {
			rows = append(rows, row{text: "暂无任务记录", bold: true}, row{text: "Pi 提交工作后会自动显示在这里。", color: muted})
		} else {
			compact := height < 23
			start, stop := 0, len(v.runs)
			if compact {
				available := max(1, limit-5)
				start = max(0, v.index-available+1)
				stop = min(len(v.runs), start+available)
			}
			for i := start; i < stop; i++ {
				run := v.runs[i]
				pointer := " "
				if i == v.index {
					pointer = "›"
				}
				symbol, state, stateColor := label(run.State)
				rows = append(rows, row{text: fmt.Sprintf("%s %s %s · %d/%d · %s", pointer, symbol, run.Title, run.DoneCount(), len(run.Tasks), state), color: stateColor, bold: i == v.index})
				if !compact {
					rows = append(rows, row{text: "    " + run.UpdatedAt.Local().Format("01-02 15:04:05") + " · " + run.Directory, color: muted}, row{})
				}
			}
		}
	} else {
		footer = "W/S ↑↓ 滚动 · A/D ←→ 翻页 · Esc 返回 · Q 退出"
		if v.current != nil {
			run := v.current
			symbol, state, stateColor := label(run.State)
			rows = append(rows, row{text: run.Title, color: accent, bold: true}, row{text: fmt.Sprintf("%s %s · 已完成 %d/%d · 更新于 %s", symbol, state, run.DoneCount(), len(run.Tasks), run.UpdatedAt.Local().Format("15:04:05")), color: stateColor})
			if !run.Closed() && v.now.Sub(run.UpdatedAt) > 5*time.Minute {
				rows = append(rows, row{text: "! 已超过 5 分钟未更新；可能仍在执行，不代表失败。", color: warning})
			}
			if height >= 12 {
				rows = append(rows, row{text: run.Directory + " · " + run.ID, color: muted})
			}
		} else {
			rows = append(rows, row{text: "等待工作记录", color: muted})
		}
		rows = append(rows, row{text: strings.Repeat("─", width), color: muted})
		body := v.body(width)
		available := max(1, limit-len(rows)-2)
		if v.readError != "" || v.warning != "" {
			available--
		}
		available = max(1, available)
		v.offset = min(v.offset, max(0, len(body)-available))
		stop := min(len(body), v.offset+available)
		rows = append(rows, body[v.offset:stop]...)
		if len(body) > available {
			rows = append(rows, row{text: fmt.Sprintf("第 %d–%d / %d 行", v.offset+1, stop, len(body)), color: muted})
		}
	}
	if width < 60 {
		if v.detailID == "" {
			footer = "W/S ↑↓ · A/D ←→ · Enter/Q"
		} else {
			footer = "W/S ↑↓ · A/D ←→ · Esc/Q"
		}
	}
	if width < 32 {
		if v.detailID == "" {
			footer = "WS/AD · Enter · Q"
		} else {
			footer = "WS/AD · Esc · Q"
		}
	}
	// Reserve a row for read errors even in a short terminal; never hide a stale-snapshot warning.
	contentLimit := limit - 1
	if v.readError != "" || v.warning != "" {
		contentLimit--
	}
	if len(rows) > contentLimit {
		rows = rows[:contentLimit]
	}
	if v.readError != "" {
		rows = append(rows, row{text: v.readError, color: danger})
	} else if v.warning != "" {
		rows = append(rows, row{text: v.warning, color: warning})
	}
	for len(rows) < limit-1 {
		rows = append(rows, row{})
	}
	rows = append(rows, row{text: footer, color: muted})
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = r.render(width, color) + "\033[K"
	}
	return strings.Join(lines, "\r\n")
}
