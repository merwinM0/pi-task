// Package store persists private, atomic snapshots. Readers never take a write lock.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"pi-task/internal/task"
)

const maxFileSize = 4 << 20

type Store struct{ Root string }

type Listing struct {
	Runs     []task.Run `json:"runs"`
	Warnings []string   `json:"warnings,omitempty"`
}

func Default() (*Store, error) {
	root := os.Getenv("PI_TASK_HOME")
	if root == "" {
		base := os.Getenv("XDG_STATE_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			base = filepath.Join(home, ".local", "state")
		}
		root = filepath.Join(base, "pi-task")
	}
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("状态目录必须是绝对路径")
	}
	return &Store{Root: root}, nil
}

func NewID(now time.Time) (string, error) {
	var bytes [6]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return now.Format("20060102-150405") + "-" + hex.EncodeToString(bytes[:]), nil
}

func (s *Store) path(id string) string { return filepath.Join(s.Root, "runs", id+".json") }

func (s *Store) lock() (func(), error) {
	if err := os.MkdirAll(filepath.Join(s.Root, "runs"), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(s.Root, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			f.Close()
			return nil, err
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("等待状态写入锁超时，请稍后重试")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
}

func (s *Store) Read(id string) (task.Run, error) {
	var run task.Run
	if err := task.ValidateID(id); err != nil {
		return run, err
	}
	f, err := os.Open(s.path(id))
	if err != nil {
		return run, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return run, err
	}
	if len(data) > maxFileSize {
		return run, fmt.Errorf("状态文件过大")
	}
	if err := json.Unmarshal(data, &run); err != nil {
		return run, fmt.Errorf("状态文件损坏：%w", err)
	}
	if err := run.Validate(); err != nil {
		return run, fmt.Errorf("状态文件无效：%w", err)
	}
	if run.ID != id {
		return run, fmt.Errorf("文件名与工作 ID 不一致")
	}
	return run, nil
}

func (s *Store) write(run task.Run) error {
	if err := run.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	if len(data)+1 > maxFileSize {
		return fmt.Errorf("状态快照超过 4 MiB，拒绝写入以免生成无法读取的记录")
	}
	f, err := os.CreateTemp(filepath.Join(s.Root, "runs"), ".snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(f.Name(), s.path(run.ID)); err != nil {
		return err
	}
	// Best-effort directory fsync: some filesystems don't support syncing directories.
	if d, err := os.Open(filepath.Dir(f.Name())); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func (s *Store) Create(id, title, directory string, texts []string) (task.Run, error) {
	now := time.Now().UTC()
	if id == "" {
		var err error
		id, err = NewID(now)
		if err != nil {
			return task.Run{}, err
		}
	}
	run := task.Run{Schema: task.SchemaVersion, ID: id, Title: title, Directory: directory,
		State: task.Running, Revision: 1, CreatedAt: now, UpdatedAt: now, Tasks: []task.Item{}}
	if len(texts) > 0 {
		if err := run.Submit(texts); err != nil {
			return run, err
		}
	}
	if err := run.Validate(); err != nil {
		return run, err
	}
	unlock, err := s.lock()
	if err != nil {
		return run, err
	}
	defer unlock()
	if _, err := os.Lstat(s.path(id)); err == nil {
		return run, fmt.Errorf("工作 ID 已存在，不会覆盖")
	} else if !errors.Is(err, os.ErrNotExist) {
		return run, err
	}
	return run, s.write(run)
}

// Change holds the lock across read/modify/write, so concurrent callers cannot lose updates.
func (s *Store) Change(id string, change func(*task.Run) error) (task.Run, error) {
	if err := task.ValidateID(id); err != nil {
		return task.Run{}, err
	}
	unlock, err := s.lock()
	if err != nil {
		return task.Run{}, err
	}
	defer unlock()
	run, err := s.Read(id)
	if err != nil {
		return run, err
	}
	if err := change(&run); err != nil {
		return run, err
	}
	run.Revision++
	run.UpdatedAt = time.Now().UTC()
	if run.UpdatedAt.Before(run.CreatedAt) {
		run.UpdatedAt = run.CreatedAt
	}
	if run.FinishedAt != nil && run.UpdatedAt.Before(*run.FinishedAt) {
		run.UpdatedAt = *run.FinishedAt
	}
	return run, s.write(run)
}

func (s *Store) Recent(limit int) (Listing, error) {
	list := Listing{Runs: []task.Run{}}
	entries, err := os.ReadDir(filepath.Join(s.Root, "runs"))
	if errors.Is(err, os.ErrNotExist) {
		return list, nil
	}
	if err != nil {
		return list, err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		run, err := s.Read(id)
		if err != nil {
			list.Warnings = append(list.Warnings, fmt.Sprintf("%s：%v", entry.Name(), err))
			continue
		}
		list.Runs = append(list.Runs, run)
	}
	sort.Slice(list.Runs, func(i, j int) bool {
		a, b := list.Runs[i], list.Runs[j]
		if a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.ID > b.ID
		}
		return a.UpdatedAt.After(b.UpdatedAt)
	})
	if limit > 0 && len(list.Runs) > limit {
		list.Runs = list.Runs[:limit]
	}
	return list, nil
}
