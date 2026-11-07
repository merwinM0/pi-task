package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"pi-task/internal/task"
)

func TestSnapshotsPermissionsAndDuplicateProtection(t *testing.T) {
	s := Store{Root: filepath.Join(t.TempDir(), "state")}
	r, err := s.Create("run-01", "测试", "/project", []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(r.ID, "覆盖", "/", nil); err == nil {
		t.Fatal("duplicate overwrote state")
	}
	changed, err := s.Change(r.ID, func(r *task.Run) error { return r.Update(1, task.Done, nil, nil, 2) })
	if err != nil {
		t.Fatal(err)
	}
	if changed.Revision != 2 {
		t.Fatal(changed.Revision)
	}
	onDisk, err := s.Read(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Tasks[0].State != task.Done {
		t.Fatal(onDisk)
	}
	for _, name := range []string{s.Root, filepath.Join(s.Root, "runs"), s.path(r.ID), filepath.Join(s.Root, ".lock")} {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("public permissions on %s: %o", name, info.Mode().Perm())
		}
	}
	entries, _ := os.ReadDir(filepath.Join(s.Root, "runs"))
	if len(entries) != 1 {
		t.Fatal("temporary snapshots leaked")
	}
}

func TestConcurrentWritersAndReaders(t *testing.T) {
	s := Store{Root: filepath.Join(t.TempDir(), "state")}
	_, err := s.Create("parallel", "并发测试", "/", []string{"initial"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 32)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.Change("parallel", func(r *task.Run) error { _, e := r.Add(fmt.Sprintf("step-%d", i)); return e })
			if err != nil {
				errors <- err
			}
			if _, err := s.Read("parallel"); err != nil {
				errors <- err
			}
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	r, err := s.Read("parallel")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tasks) != 21 || r.Revision != 21 {
		t.Fatal("lost update", len(r.Tasks), r.Revision)
	}
}

func TestRecentFiveAndCorruptRecords(t *testing.T) {
	s := Store{Root: filepath.Join(t.TempDir(), "state")}
	for i := 0; i < 7; i++ {
		if _, err := s.Create(fmt.Sprintf("run-%d", i), "测试", "/", nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(s.path("corrupt"), []byte("{bad json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Change("run-0", func(r *task.Run) error { _, e := r.Add("new update"); return e }); err != nil {
		t.Fatal(err)
	}
	list, err := s.Recent(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Runs) != 5 || list.Runs[0].ID != "run-0" || len(list.Warnings) != 1 {
		t.Fatal(list)
	}
	if _, err := s.Read("../escape"); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestFailedChangesLeaveRevisionAndSnapshotUntouched(t *testing.T) {
	s := Store{Root: filepath.Join(t.TempDir(), "state")}
	_, err := s.Create("one", "测试", "/", []string{"pending"})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path("one"))
	_, err = s.Change("one", func(r *task.Run) error { r.Title = "partially changed"; return fmt.Errorf("failure") })
	if err == nil {
		t.Fatal("expected error")
	}
	after, _ := os.ReadFile(s.path("one"))
	if string(before) != string(after) {
		t.Fatal("failed change leaked into snapshot")
	}
	_, err = s.Change("one", func(r *task.Run) error { r.Tasks[0].State = "invalid"; return nil })
	if err == nil {
		t.Fatal("invalid snapshot written")
	}
}

func TestOversizeWriteDoesNotDestroyReadableSnapshot(t *testing.T) {
	s := Store{Root: t.TempDir()}
	if _, err := s.Create("small", "测试", "/", nil); err != nil {
		t.Fatal(err)
	}
	_, err := s.Change("small", func(r *task.Run) error {
		r.Directory = strings.Repeat("x", maxFileSize)
		return nil
	})
	if err == nil {
		t.Fatal("oversized write accepted")
	}
	run, err := s.Read("small")
	if err != nil || run.Revision != 1 {
		t.Fatal("old snapshot lost", run.Revision, err)
	}
}

func TestReadOnlyEmptyListingDoesNotCreateDirectory(t *testing.T) {
	s := Store{Root: filepath.Join(t.TempDir(), "absent")}
	list, err := s.Recent(5)
	if err != nil || len(list.Runs) != 0 {
		t.Fatal(list, err)
	}
	if _, err := os.Stat(s.Root); !os.IsNotExist(err) {
		t.Fatal("viewer created storage")
	}
}
