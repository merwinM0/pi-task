// Package cli exposes small, explicit writes for agents and a name-free viewer for humans.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"pi-task/internal/store"
)

const Version = "0.1.0"

const help = `pi-task — 独立任务进度监看器（不执行任务）

查看：
  pi-task                       最近 5 次工作，W/S 或 ↑/↓ 选择，Enter 实时查看
  pi-task status                同上；非终端环境输出静态列表
  pi-task status --once          输出最近 5 次工作，不打开交互界面
  pi-task status --json          输出最近 5 次工作的 JSON
  pi-task status --id ID         直接查看指定工作的实时进度

提交：
  pi-task start --title 标题 [--task 步骤 ...] [--id ID]
  pi-task start --title 标题 --file plan.json
  pi-task submit --id ID --task 步骤 --task 步骤
  pi-task submit --id ID --file -        从标准输入读取 JSON 字符串数组
  pi-task add --id ID --text 新步骤
  pi-task update --id ID --task 1 --state running
  pi-task update --id ID --task 1 --state done --next 2 --note 验证依据
  pi-task update --id ID --task 2 --text 修订标题
  pi-task finish --id ID [--state completed|failed|cancelled]

步骤状态：pending / running / done / blocked / cancelled
start 返回工作 ID；后续写入必须显式指定 ID，避免多个 Pi 会话串线。
finish 不会自动给步骤打勾；completed 要求所有步骤确实已标记 done。

界面：W/S 或 ↑/↓ 移动 · Enter 查看 · Esc 返回 · Q/Ctrl+C 退出
环境：PI_TASK_HOME 覆盖状态目录；NO_COLOR 禁用颜色。
无需服务、端口、API 密钥或 Pi 插件。仅支持 Unix 终端。
`

type app struct {
	store *store.Store
	in    *os.File
	out   io.Writer
	err   io.Writer
}

func Run(args []string, in *os.File, out, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "help", "--help", "-h":
			fmt.Fprint(out, help)
			return 0
		case "version", "--version", "-version":
			fmt.Fprintln(out, Version)
			return 0
		}
	}
	s, err := store.Default()
	if err != nil {
		fmt.Fprintln(stderr, "pi-task:", err)
		return 1
	}
	a := app{store: s, in: in, out: out, err: stderr}
	command := "status"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	var code int
	switch command {
	case "start":
		err = a.start(args)
	case "submit":
		err = a.submit(args)
	case "add":
		err = a.add(args)
	case "update":
		err = a.update(args)
	case "finish":
		err = a.finish(args)
	case "status":
		code, err = a.status(args)
	default:
		err = fmt.Errorf("未知命令 %q；运行 pi-task --help 查看用法", command)
	}
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(out, help)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "pi-task:", err)
		return 1
	}
	return code
}

func flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	return f
}

func parse(f *flag.FlagSet, args []string) error {
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return fmt.Errorf("多余参数 %q；标题和步骤请用引号包围，并通过对应选项传入", f.Arg(0))
	}
	return nil
}

func requiredID(id string) error {
	if id == "" {
		return fmt.Errorf("写入必须提供 --id；请使用 start 返回的工作 ID")
	}
	return nil
}
