package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/term"
	"pi-task/internal/ui"
)

func (a app) status(args []string) (int, error) {
	f := flags("status")
	id := f.String("id", "", "直接查看指定工作")
	once := f.Bool("once", false, "静态输出")
	jsonOutput := f.Bool("json", false, "JSON 输出")
	watch := f.Bool("watch", false, "实时查看，终端中默认开启")
	if err := parse(f, args); err != nil {
		return 0, err
	}
	if *watch && (*once || *jsonOutput) {
		return 0, fmt.Errorf("--watch 不能与 --once/--json 同时使用")
	}
	out, isFile := a.out.(*os.File)
	interactive := isFile && term.IsTerminal(int(out.Fd())) && term.IsTerminal(int(a.in.Fd())) && os.Getenv("TERM") != "dumb"
	if *watch && !interactive {
		return 0, fmt.Errorf("实时查看需要交互终端；脚本中请使用 --once 或 --json")
	}
	if interactive && !*once && !*jsonOutput {
		if *id != "" {
			if _, err := a.store.Read(*id); err != nil {
				return 0, err
			}
		}
		return ui.Run(a.store, a.in, out, *id)
	}
	encoder := json.NewEncoder(a.out)
	encoder.SetIndent("", "  ")
	if *id != "" {
		run, err := a.store.Read(*id)
		if err != nil {
			return 0, err
		}
		if *jsonOutput {
			return 0, encoder.Encode(run)
		}
		_, err = fmt.Fprintln(a.out, ui.PlainRun(run))
		return 0, err
	}
	list, err := a.store.Recent(5)
	if err != nil {
		return 0, err
	}
	if *jsonOutput {
		return 0, encoder.Encode(list)
	}
	for _, warning := range list.Warnings {
		fmt.Fprintln(a.err, "警告：", ui.Clean(warning))
	}
	_, err = fmt.Fprintln(a.out, ui.PlainList(list.Runs))
	return 0, err
}
