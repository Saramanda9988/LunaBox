package env

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// StorageFile 是 Storage 在数据目录里的文件名。
const StorageFile = "storage.json"

// Storage 对应原版的 localStorage：界面状态（各页面的搜索、排序与筛选）存进数据目录的 storage.json，
// 重启后恢复。值按键保存为 JSON；写入在后台进行，连续的修改合并成一次写入。path 为空（测试）时只在内存里。
type Storage struct {
	path string

	mu      sync.Mutex
	values  map[string]json.RawMessage
	dirty   bool
	writing bool
}

// NewStorage 读取 dir 下的 storage.json；dir 为空时不读写文件。文件不存在或损坏时从空开始。
func NewStorage(dir string) *Storage {
	s := &Storage{values: map[string]json.RawMessage{}}
	if dir == "" {
		return s
	}
	s.path = filepath.Join(dir, StorageFile)
	data, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("ui storage: read: %v", err)
		}
		return s
	}
	if err := json.Unmarshal(data, &s.values); err != nil {
		log.Printf("ui storage: parse: %v", err)
		s.values = map[string]json.RawMessage{}
	}
	// 文件内容是 null 时 Unmarshal 不报错但把 map 置为 nil，之后 Set 会 panic。
	if s.values == nil {
		s.values = map[string]json.RawMessage{}
	}
	return s
}

// Get 把 key 的值解码进 v，没有保存过或解码失败时返回 false。s 为 nil 时也返回 false。
func (s *Storage) Get(key string, v any) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	raw, ok := s.values[key]
	s.mu.Unlock()
	return ok && json.Unmarshal(raw, v) == nil
}

// Set 保存 key 的值并在后台写入文件。s 为 nil 时什么也不做。
func (s *Storage) Set(key string, v any) {
	if s == nil {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		log.Printf("ui storage: encode %s: %v", key, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = raw
	if s.path == "" {
		return
	}
	s.dirty = true
	if !s.writing {
		s.writing = true
		go s.flush()
	}
}

// flush 在后台写文件，写的过程中又有修改时再写一次。
func (s *Storage) flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for s.dirty {
		s.dirty = false
		data, err := json.MarshalIndent(s.values, "", "  ")
		if err != nil {
			log.Printf("ui storage: encode: %v", err)
			break
		}
		s.mu.Unlock()
		err = writeFileAtomic(s.path, data)
		s.mu.Lock()
		if err != nil {
			log.Printf("ui storage: write: %v", err)
		}
	}
	s.writing = false
}

// writeFileAtomic 先写临时文件再改名，写到一半退出时不会留下损坏的文件。
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
