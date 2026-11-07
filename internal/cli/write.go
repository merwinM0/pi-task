package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"pi-task/internal/task"
)

type texts []string

func (t *texts) String() string         { return strings.Join(*t, ", ") }
func (t *texts) Set(value string) error { *t = append(*t, value); return nil }

func (a app) plan(file string, values texts, required bool) ([]string, error) {
	if file != "" && len(values) > 0 {
		return nil, fmt.Errorf("--file 与 --task 不能同时使用")
	}
	if file != "" {
		var reader io.Reader = a.in
		if file != "-" {
			f, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			reader = f
		}
		data, err := io.ReadAll(io.LimitReader(reader, (1<<20)+1))
		if err != nil {
			return nil, err
		}
		if len(data) > 1<<20 {
			return nil, fmt.Errorf("清单文件超过 1 MiB")
		}
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, fmt.Errorf("清单格式应为 JSON 字符串数组：%w", err)
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("清单不能为空")
		}
	}
	if required && len(values) == 0 {
		return nil, fmt.Errorf("请通过 --task 或 --file 提交任务清单")
	}
	return values, nil
}

func (a app) start(args []string) error {
	f := flags("start")
	title := f.String("title", "", "工作标题")
	id := f.String("id", "", "可选工作 ID")
	file := f.String("file", "", "JSON 清单文件，- 表示标准输入")
	var values texts
	f.Var(&values, "task", "步骤标题，可重复")
	if err := parse(f, args); err != nil {
		return err
	}
	plan, err := a.plan(*file, values, false)
	if err != nil {
		return err
	}
	directory, err := os.Getwd()
	if err != nil {
		return err
	}
	run, err := a.store.Create(*id, strings.TrimSpace(*title), directory, plan)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(a.out, run.ID)
	return err
}

func (a app) submit(args []string) error {
	f := flags("submit")
	id := f.String("id", "", "工作 ID")
	file := f.String("file", "", "清单文件")
	var values texts
	f.Var(&values, "task", "步骤标题，可重复")
	if err := parse(f, args); err != nil {
		return err
	}
	if err := requiredID(*id); err != nil {
		return err
	}
	plan, err := a.plan(*file, values, true)
	if err != nil {
		return err
	}
	run, err := a.store.Change(*id, func(r *task.Run) error { return r.Submit(plan) })
	if err != nil {
		return err
	}
	return a.confirm(run)
}

func (a app) add(args []string) error {
	f := flags("add")
	id := f.String("id", "", "工作 ID")
	text := f.String("text", "", "步骤标题")
	if err := parse(f, args); err != nil {
		return err
	}
	if err := requiredID(*id); err != nil {
		return err
	}
	var itemID int
	_, err := a.store.Change(*id, func(r *task.Run) error {
		var e error
		itemID, e = r.Add(strings.TrimSpace(*text))
		return e
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(a.out, itemID)
	return err
}

func (a app) update(args []string) error {
	f := flags("update")
	id := f.String("id", "", "工作 ID")
	item := f.Int("task", 0, "步骤 ID")
	state := f.String("state", "", "步骤状态")
	text := f.String("text", "", "修订步骤标题")
	note := f.String("note", "", "完成依据或阻塞原因；空字符串可清除")
	next := f.Int("next", 0, "在同一次写入中开始下一步骤")
	if err := parse(f, args); err != nil {
		return err
	}
	if err := requiredID(*id); err != nil {
		return err
	}
	var textValue, noteValue *string
	f.Visit(func(option *flag.Flag) {
		if option.Name == "text" {
			trimmed := strings.TrimSpace(*text)
			textValue = &trimmed
		}
		if option.Name == "note" {
			trimmed := strings.TrimSpace(*note)
			noteValue = &trimmed
		}
	})
	run, err := a.store.Change(*id, func(r *task.Run) error {
		return r.Update(*item, task.State(*state), textValue, noteValue, *next)
	})
	if err != nil {
		return err
	}
	return a.confirm(run)
}

func (a app) finish(args []string) error {
	f := flags("finish")
	id := f.String("id", "", "工作 ID")
	state := f.String("state", "completed", "结束状态")
	if err := parse(f, args); err != nil {
		return err
	}
	if err := requiredID(*id); err != nil {
		return err
	}
	run, err := a.store.Change(*id, func(r *task.Run) error { return r.Finish(task.State(*state), time.Now().UTC()) })
	if err != nil {
		return err
	}
	return a.confirm(run)
}

func (a app) confirm(run task.Run) error {
	_, err := fmt.Fprintf(a.out, "ok %s rev=%d\n", run.ID, run.Revision)
	return err
}
