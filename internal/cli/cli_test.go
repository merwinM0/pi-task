package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pi-task/internal/task"
)

func invoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	in, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	var out, stderr bytes.Buffer
	code := Run(args, in, &out, &stderr)
	return code, out.String(), stderr.String()
}

func TestCLIWorkflowAndPlainOutput(t *testing.T) {
	t.Setenv("PI_TASK_HOME", filepath.Join(t.TempDir(), "state"))
	code, out, errText := invoke(t, "start", "--title", "修复页面", "--id", "demo", "--task", "检查", "--task", "验证")
	if code != 0 || out != "demo\n" {
		t.Fatal(code, out, errText)
	}
	for _, args := range [][]string{
		{"update", "--id", "demo", "--task", "1", "--state", "done", "--next", "2"},
		{"update", "--id", "demo", "--task", "2", "--state", "done", "--note", "测试通过"},
		{"finish", "--id", "demo"},
	} {
		if c, o, e := invoke(t, args...); c != 0 {
			t.Fatal(c, o, e)
		}
	}
	code, out, errText = invoke(t, "status", "--id", "demo", "--json")
	var run task.Run
	if code != 0 || json.Unmarshal([]byte(out), &run) != nil || run.State != task.Completed {
		t.Fatal(code, out, errText)
	}
	code, out, errText = invoke(t)
	if code != 0 || !strings.Contains(out, "修复页面") || strings.Contains(out, "\033") {
		t.Fatal(code, out, errText)
	}
}

func TestCLIErrorsAreNonzeroAndDoNotCompleteTasks(t *testing.T) {
	t.Setenv("PI_TASK_HOME", filepath.Join(t.TempDir(), "state"))
	_, _, _ = invoke(t, "start", "--title", "任务", "--id", "run", "--task", "unfinished")
	for _, args := range [][]string{
		{"finish", "--id", "run"},
		{"update", "--task", "1", "--state", "done"},
		{"update", "--id", "run", "--task", "9", "--state", "done"},
		{"submit", "--id", "run", "--task", "replacement"},
		{"status", "--watch"},
		{"start", "--title", "bad\033[31m"},
		{"start", "--title", "okay", "unexpected"},
	} {
		code, _, errText := invoke(t, args...)
		if code == 0 || errText == "" {
			t.Fatalf("accepted %v: %s", args, errText)
		}
	}
	code, out, _ := invoke(t, "status", "--id", "run", "--json")
	var run task.Run
	if code != 0 || json.Unmarshal([]byte(out), &run) != nil || run.Tasks[0].State != task.Pending {
		t.Fatal(out)
	}
}

func TestFilePlanAndHelp(t *testing.T) {
	t.Setenv("PI_TASK_HOME", filepath.Join(t.TempDir(), "state"))
	file := filepath.Join(t.TempDir(), "plan.json")
	_ = os.WriteFile(file, []byte(`["中文步骤","emoji 🚀"]`), 0600)
	if code, out, e := invoke(t, "start", "--title", "导入", "--file", file); code != 0 || strings.TrimSpace(out) == "" {
		t.Fatal(code, out, e)
	}
	if code, _, _ := invoke(t, "start", "--title", "混用", "--file", file, "--task", "extra"); code == 0 {
		t.Fatal("ambiguous input accepted")
	}
	if code, out, _ := invoke(t, "--help"); code != 0 || !strings.Contains(out, "最近 5") {
		t.Fatal(code, out)
	}
	if code, out, _ := invoke(t, "--version"); code != 0 || !strings.Contains(out, Version) {
		t.Fatal(code, out)
	}
}
