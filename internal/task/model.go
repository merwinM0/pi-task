// Package task defines reported progress, not a scheduler or an executor.
package task

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const SchemaVersion = 1
const MaxTasks = 500

var validID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$`)

func ValidateID(id string) error {
	if !validID.MatchString(id) {
		return fmt.Errorf("工作 ID 需为 1–80 位字母、数字、下划线或短横线，且以字母或数字开头")
	}
	return nil
}

type State string

const (
	Pending   State = "pending"
	Running   State = "running"
	Done      State = "done"
	Blocked   State = "blocked"
	Cancelled State = "cancelled"
	Completed State = "completed"
	Failed    State = "failed"
)

type Item struct {
	ID    int    `json:"id"`
	Text  string `json:"text"`
	State State  `json:"state"`
	Note  string `json:"note,omitempty"`
}

type Run struct {
	Schema     int        `json:"schema"`
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Directory  string     `json:"directory"`
	State      State      `json:"state"`
	Revision   uint64     `json:"revision"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Tasks      []Item     `json:"tasks"`
}

func ValidateText(text string, max int) error {
	if strings.TrimSpace(text) == "" || !utf8.ValidString(text) || utf8.RuneCountInString(text) > max {
		return fmt.Errorf("文本不能为空，必须为有效 UTF-8，且不超过 %d 个字符", max)
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return fmt.Errorf("文本不能包含换行、制表符或终端控制字符")
		}
	}
	return nil
}

func ValidItemState(state State) bool {
	return state == Pending || state == Running || state == Done || state == Blocked || state == Cancelled
}

func (r Run) Closed() bool { return r.State != Running }

func (r Run) DoneCount() int {
	n := 0
	for _, t := range r.Tasks {
		if t.State == Done {
			n++
		}
	}
	return n
}

func (r Run) Validate() error {
	if r.Schema != SchemaVersion {
		return fmt.Errorf("不支持的数据版本 %d", r.Schema)
	}
	if err := ValidateID(r.ID); err != nil {
		return err
	}
	if err := ValidateText(r.Title, 200); err != nil {
		return fmt.Errorf("工作标题：%w", err)
	}
	if r.State != Running && r.State != Completed && r.State != Failed && r.State != Cancelled {
		return fmt.Errorf("无效工作状态 %q", r.State)
	}
	if r.Revision == 0 || r.CreatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) {
		return fmt.Errorf("工作元数据无效")
	}
	if r.Closed() != (r.FinishedAt != nil) {
		return fmt.Errorf("结束时间与工作状态不一致")
	}
	if r.FinishedAt != nil && (r.FinishedAt.Before(r.CreatedAt) || r.FinishedAt.After(r.UpdatedAt)) {
		return fmt.Errorf("结束时间无效")
	}
	if len(r.Tasks) > MaxTasks {
		return fmt.Errorf("最多支持 %d 个步骤", MaxTasks)
	}
	seen := make(map[int]bool)
	for _, t := range r.Tasks {
		if t.ID <= 0 || seen[t.ID] {
			return fmt.Errorf("步骤 ID 必须为不重复的正整数")
		}
		seen[t.ID] = true
		if err := ValidateText(t.Text, 500); err != nil {
			return fmt.Errorf("步骤 %d：%w", t.ID, err)
		}
		if !ValidItemState(t.State) {
			return fmt.Errorf("步骤 %d 状态无效", t.ID)
		}
		if t.Note != "" {
			if err := ValidateText(t.Note, 2000); err != nil {
				return fmt.Errorf("步骤 %d 备注：%w", t.ID, err)
			}
		}
	}
	if r.State == Completed && (len(r.Tasks) == 0 || r.DoneCount() != len(r.Tasks)) {
		return fmt.Errorf("尚有未完成步骤，不能标记工作已完成")
	}
	return nil
}

func (r *Run) Submit(texts []string) error {
	if r.Closed() {
		return fmt.Errorf("工作已经结束")
	}
	if len(r.Tasks) != 0 {
		return fmt.Errorf("清单已存在；请用 add 增补、update 修订，避免覆盖既有进度")
	}
	if len(texts) == 0 || len(texts) > MaxTasks {
		return fmt.Errorf("清单需要 1–%d 个步骤", MaxTasks)
	}
	items := make([]Item, len(texts))
	for i, text := range texts {
		text = strings.TrimSpace(text)
		if err := ValidateText(text, 500); err != nil {
			return err
		}
		items[i] = Item{ID: i + 1, Text: text, State: Pending}
	}
	r.Tasks = items
	return nil
}

// Update is atomic when used inside Store.Change. next marks the following step
// running in the same write, without inferring completion from any external tool.
func (r *Run) Update(id int, state State, text, note *string, next int) error {
	if r.Closed() {
		return fmt.Errorf("工作已经结束，不能继续更新")
	}
	index, nextIndex := -1, -1
	for i := range r.Tasks {
		if r.Tasks[i].ID == id {
			index = i
		}
		if r.Tasks[i].ID == next {
			nextIndex = i
		}
	}
	if index < 0 {
		return fmt.Errorf("找不到步骤 %d", id)
	}
	if state != "" && !ValidItemState(state) {
		return fmt.Errorf("无效步骤状态 %q", state)
	}
	if state == "" && text == nil && note == nil {
		return fmt.Errorf("没有提供状态、标题或备注更新")
	}
	if next != 0 {
		if nextIndex < 0 || next == id {
			return fmt.Errorf("--next 需指定另一个已存在的步骤")
		}
		if state != Done {
			return fmt.Errorf("--next 仅能与 --state done 一起使用")
		}
		if r.Tasks[nextIndex].State == Done || r.Tasks[nextIndex].State == Cancelled {
			return fmt.Errorf("下一步骤已经结束")
		}
	}
	if text != nil {
		if err := ValidateText(*text, 500); err != nil {
			return err
		}
	}
	if note != nil && *note != "" {
		if err := ValidateText(*note, 2000); err != nil {
			return err
		}
	}
	if state != "" {
		r.Tasks[index].State = state
	}
	if text != nil {
		r.Tasks[index].Text = *text
	}
	if note != nil {
		r.Tasks[index].Note = *note
	}
	if next != 0 {
		r.Tasks[nextIndex].State = Running
	}
	return nil
}

func (r *Run) Add(text string) (int, error) {
	if r.Closed() {
		return 0, fmt.Errorf("工作已经结束")
	}
	if len(r.Tasks) >= MaxTasks {
		return 0, fmt.Errorf("步骤数量已达上限")
	}
	if err := ValidateText(text, 500); err != nil {
		return 0, err
	}
	id := 1
	for _, item := range r.Tasks {
		if item.ID >= id {
			id = item.ID + 1
		}
	}
	r.Tasks = append(r.Tasks, Item{ID: id, Text: text, State: Pending})
	return id, nil
}

func (r *Run) Finish(state State, now time.Time) error {
	if r.Closed() {
		return fmt.Errorf("工作已经结束")
	}
	if state != Completed && state != Failed && state != Cancelled {
		return fmt.Errorf("结束状态只能是 completed、failed 或 cancelled")
	}
	if state == Completed && (len(r.Tasks) == 0 || r.DoneCount() != len(r.Tasks)) {
		return fmt.Errorf("尚有未完成步骤；请核实后更新，或以 failed/cancelled 结束")
	}
	r.State, r.FinishedAt = state, &now
	return nil
}
