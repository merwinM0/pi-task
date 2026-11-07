package task

import (
	"testing"
	"time"
)

func fixture() Run {
	now := time.Now().UTC()
	return Run{Schema: SchemaVersion, ID: "test-01", Title: "中文任务", State: Running, Revision: 1, CreatedAt: now, UpdatedAt: now, Tasks: []Item{}}
}

func TestLifecycle(t *testing.T) {
	r := fixture()
	if err := r.Submit([]string{"检查代码", "修改代码", "运行测试"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Finish(Completed, time.Now()); err == nil {
		t.Fatal("must not silently complete pending steps")
	}
	if err := r.Update(1, Done, nil, nil, 2); err != nil {
		t.Fatal(err)
	}
	if r.Tasks[0].State != Done || r.Tasks[1].State != Running {
		t.Fatal(r.Tasks)
	}
	note := "30 项测试通过"
	if err := r.Update(2, Done, nil, &note, 3); err != nil {
		t.Fatal(err)
	}
	if err := r.Update(3, Done, nil, nil, 0); err != nil {
		t.Fatal(err)
	}
	if err := r.Finish(Completed, time.Now()); err != nil {
		t.Fatal(err)
	}
	r.UpdatedAt = time.Now()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := r.Update(1, Running, nil, nil, 0); err == nil {
		t.Fatal("closed run was editable")
	}
}

func TestSubmitAndValidation(t *testing.T) {
	for _, id := range []string{"../bad", "a/b", "", ".hidden", "hello world", "中文"} {
		if ValidateID(id) == nil {
			t.Errorf("accepted unsafe ID %q", id)
		}
	}
	for _, text := range []string{"", "   ", "bad\033[31m", "line\nline", string([]byte{0xff})} {
		if ValidateText(text, 20) == nil {
			t.Errorf("accepted unsafe text %q", text)
		}
	}
	r := fixture()
	if err := r.Submit([]string{"okay", "bad\nline"}); err == nil {
		t.Fatal("invalid plan accepted")
	}
	if len(r.Tasks) != 0 {
		t.Fatal("partial submit leaked into state")
	}
	if err := r.Submit([]string{"okay"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Submit([]string{"replacement"}); err == nil {
		t.Fatal("replaced original plan")
	}
}

func TestBadUpdateIsAtomic(t *testing.T) {
	r := fixture()
	_ = r.Submit([]string{"first", "second"})
	if err := r.Update(1, Done, nil, nil, 99); err == nil {
		t.Fatal("invalid next ID accepted")
	}
	if r.Tasks[0].State != Pending {
		t.Fatal("partial mutation")
	}
	if err := r.Update(1, Running, nil, nil, 2); err == nil {
		t.Fatal("next requires completion")
	}
	if err := r.Update(1, "", nil, nil, 0); err == nil {
		t.Fatal("empty update accepted")
	}
}

func TestAddAndFailedFinishPreserveIncompleteItems(t *testing.T) {
	r := fixture()
	_ = r.Submit([]string{"检查"})
	id, err := r.Add("修复")
	if err != nil || id != 2 {
		t.Fatal(id, err)
	}
	_ = r.Update(1, Blocked, nil, nil, 0)
	if err := r.Finish(Failed, time.Now()); err != nil {
		t.Fatal(err)
	}
	if r.DoneCount() != 0 || r.Tasks[0].State != Blocked {
		t.Fatal("finish changed item states")
	}
}
