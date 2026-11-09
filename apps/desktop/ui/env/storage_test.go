package env

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// waitWritten 等后台写入结束。
func waitWritten(t *testing.T, s *Storage) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		done := !s.writing
		s.mu.Unlock()
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("storage write did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

// 保存的值写进数据目录的 storage.json，重新打开（重启）后能读回来。
func TestStoragePersists(t *testing.T) {
	dir := t.TempDir()
	type value struct{ A string }
	s := NewStorage(dir)
	if s.Get("k", new(value)) {
		t.Fatal("empty storage has k")
	}
	s.Set("k", value{"1"})
	s.Set("k", value{"2"})
	s.Set("other", 3)
	waitWritten(t, s)

	var got value
	if !NewStorage(dir).Get("k", &got) || got.A != "2" {
		t.Fatalf("reopened k = %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, StorageFile+".tmp")); !os.IsNotExist(err) {
		t.Errorf("temp file left behind: %v", err)
	}
}

// 文件损坏时从空开始；没有数据目录或 nil 时只在内存里（或什么也不做）。
func TestStorageFallbacks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, StorageFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	var n int
	if NewStorage(dir).Get("k", &n) {
		t.Error("corrupt file returned a value")
	}

	mem := NewStorage("")
	mem.Set("k", 1)
	if !mem.Get("k", &n) || n != 1 {
		t.Errorf("memory storage k = %d", n)
	}

	var nilStorage *Storage
	nilStorage.Set("k", 1)
	if nilStorage.Get("k", &n) {
		t.Error("nil storage returned a value")
	}
}
