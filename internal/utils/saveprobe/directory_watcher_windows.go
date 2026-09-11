//go:build windows

package saveprobe

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

const directoryChangeBufferSize = 64 * 1024

type directoryWatcher struct {
	root          string
	gameDirectory string
	handle        windows.Handle
	store         *activityStore
	done          chan struct{}
	finished      chan struct{}
	stopOnce      sync.Once
	errMu         sync.Mutex
	err           error
}

func startDirectoryWatcher(root string, gameDirectory string, store *activityStore) (*directoryWatcher, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect game directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("game directory is not a directory: %s", root)
	}

	rootUTF16, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return nil, fmt.Errorf("encode game directory: %w", err)
	}
	handle, err := windows.CreateFile(
		rootUTF16,
		windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("open game directory monitor: %w", err)
	}

	watcher := &directoryWatcher{
		root:          root,
		gameDirectory: gameDirectory,
		handle:        handle,
		store:         store,
		done:          make(chan struct{}),
		finished:      make(chan struct{}),
	}
	go watcher.run()
	return watcher, nil
}

func (w *directoryWatcher) run() {
	defer close(w.finished)
	buffer := make([]byte, directoryChangeBufferSize)
	const notifyMask = windows.FILE_NOTIFY_CHANGE_FILE_NAME |
		windows.FILE_NOTIFY_CHANGE_DIR_NAME |
		windows.FILE_NOTIFY_CHANGE_SIZE |
		windows.FILE_NOTIFY_CHANGE_LAST_WRITE |
		windows.FILE_NOTIFY_CHANGE_CREATION

	for {
		var bytesReturned uint32
		err := windows.ReadDirectoryChanges(
			w.handle,
			&buffer[0],
			uint32(len(buffer)),
			true,
			notifyMask,
			&bytesReturned,
			nil,
			0,
		)
		if err != nil {
			select {
			case <-w.done:
				return
			default:
				w.setError(fmt.Errorf("monitor game directory: %w", err))
				return
			}
		}
		if bytesReturned == 0 {
			continue
		}
		w.processBuffer(buffer[:bytesReturned])
	}
}

func (w *directoryWatcher) processBuffer(buffer []byte) {
	for offset := 0; offset+12 <= len(buffer); {
		nextOffset := int(binary.LittleEndian.Uint32(buffer[offset:]))
		action := binary.LittleEndian.Uint32(buffer[offset+4:])
		nameLength := int(binary.LittleEndian.Uint32(buffer[offset+8:]))
		nameStart := offset + 12
		nameEnd := nameStart + nameLength
		if nameLength < 0 || nameLength%2 != 0 || nameEnd > len(buffer) {
			return
		}

		nameWords := make([]uint16, nameLength/2)
		for index := range nameWords {
			nameWords[index] = binary.LittleEndian.Uint16(buffer[nameStart+index*2:])
		}
		path := filepath.Join(w.root, string(utf16.Decode(nameWords)))
		if !shouldTrackDirectoryEvent(path, w.gameDirectory) {
			if nextOffset == 0 {
				return
			}
			offset += nextOffset
			continue
		}
		now := time.Now()
		switch action {
		case windows.FILE_ACTION_ADDED:
			w.store.recordDirectoryChange(path, 12, now)
		case windows.FILE_ACTION_MODIFIED:
			w.store.recordDirectoryChange(path, 16, now)
		case windows.FILE_ACTION_RENAMED_NEW_NAME:
			w.store.recordDirectoryChange(path, 27, now)
		}

		if nextOffset == 0 {
			return
		}
		offset += nextOffset
	}
}

func shouldTrackDirectoryEvent(path string, gameDirectory string) bool {
	if isUnderDirectory(path, gameDirectory) {
		return true
	}
	lowerPath := strings.ToLower(path)
	extension := strings.ToLower(filepath.Ext(path))
	if isLikelySaveExtension(extension) {
		return true
	}
	for _, marker := range []string{
		`\save`, `\saved games\`, `\save games\`, `\my games\`, `\userdata\`,
		`\user_data\`, `\profile\`, `\progress\`, `\record\`,
	} {
		if strings.Contains(lowerPath, marker) {
			return true
		}
	}
	fileName := strings.TrimSuffix(strings.ToLower(filepath.Base(path)), extension)
	for _, keyword := range []string{"save", "sav", "slot", "record", "progress", "checkpoint", "profile", "global", "system"} {
		if strings.Contains(fileName, keyword) {
			return true
		}
	}
	return false
}

func (w *directoryWatcher) stop() error {
	w.stopOnce.Do(func() {
		close(w.done)
		_ = windows.CancelIoEx(w.handle, nil)
		_ = windows.CloseHandle(w.handle)
		select {
		case <-w.finished:
		case <-time.After(2 * time.Second):
			w.setError(errors.New("game directory monitor did not stop"))
		}
	})
	w.errMu.Lock()
	defer w.errMu.Unlock()
	return w.err
}

func (w *directoryWatcher) setError(err error) {
	w.errMu.Lock()
	w.err = errors.Join(w.err, err)
	w.errMu.Unlock()
}
