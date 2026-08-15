package steamutils

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	binaryVDFObject byte = iota
	binaryVDFString
	binaryVDFInt32
	binaryVDFFloat32
	binaryVDFPointer
	binaryVDFWideString
	binaryVDFColor
	binaryVDFUint64
	binaryVDFEnd
)

type binaryVDFEntry struct {
	Type     byte
	Key      string
	String   string
	Raw      []byte
	Children []binaryVDFEntry
}

// ShortcutFile preserves every supported field from Steam's shortcuts.vdf
// while exposing only the operations needed by the integration layer.
type ShortcutFile struct {
	entries []binaryVDFEntry
}

func NewShortcutFile() *ShortcutFile {
	return &ShortcutFile{}
}

func ParseShortcutFile(data []byte) (*ShortcutFile, error) {
	entries, err := parseBinaryVDF(data)
	if err != nil {
		return nil, err
	}
	return &ShortcutFile{entries: entries}, nil
}

func (f *ShortcutFile) Find(executable string, launchID string) (uint32, bool) {
	if f == nil {
		return 0, false
	}
	return findSteamShortcut(f.entries, executable, launchID)
}

func (f *ShortcutFile) SetLaunchOptions(executable string, launchID string, launchOptions string) (uint32, bool) {
	if f == nil {
		return 0, false
	}
	return setSteamShortcutLaunchOptions(f.entries, executable, launchID, launchOptions)
}

func (f *ShortcutFile) Add(name string, executable string, launchOptions string) (uint32, error) {
	if f == nil {
		return 0, fmt.Errorf("Steam shortcut file is nil")
	}
	entries, appID, err := appendSteamShortcut(f.entries, name, executable, launchOptions)
	if err != nil {
		return 0, err
	}
	f.entries = entries
	return appID, nil
}

func (f *ShortcutFile) MarshalBinary() ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("Steam shortcut file is nil")
	}
	return encodeBinaryVDF(f.entries)
}

func ShortcutLongID(appID uint32) string {
	return steamShortcutLongID(appID)
}

func ShortcutAppIDFromLongID(value string) (uint32, bool) {
	return steamShortcutAppIDFromLongID(value)
}

func parseBinaryVDF(data []byte) ([]binaryVDFEntry, error) {
	offset := 0
	entries, err := parseBinaryVDFEntries(data, &offset)
	if err != nil {
		return nil, err
	}
	if offset != len(data) {
		return nil, fmt.Errorf("binary VDF contains %d trailing bytes", len(data)-offset)
	}
	return entries, nil
}

func parseBinaryVDFEntries(data []byte, offset *int) ([]binaryVDFEntry, error) {
	entries := make([]binaryVDFEntry, 0)
	for {
		if *offset >= len(data) {
			return nil, fmt.Errorf("binary VDF ended before object terminator")
		}
		valueType := data[*offset]
		*offset++
		if valueType == binaryVDFEnd {
			return entries, nil
		}

		key, err := readBinaryVDFCString(data, offset)
		if err != nil {
			return nil, fmt.Errorf("read binary VDF key: %w", err)
		}
		entry := binaryVDFEntry{Type: valueType, Key: key}
		switch valueType {
		case binaryVDFObject:
			entry.Children, err = parseBinaryVDFEntries(data, offset)
		case binaryVDFString:
			entry.String, err = readBinaryVDFCString(data, offset)
		case binaryVDFInt32, binaryVDFFloat32, binaryVDFPointer, binaryVDFColor:
			entry.Raw, err = readBinaryVDFBytes(data, offset, 4)
		case binaryVDFUint64:
			entry.Raw, err = readBinaryVDFBytes(data, offset, 8)
		case binaryVDFWideString:
			err = fmt.Errorf("wide strings are not supported")
		default:
			err = fmt.Errorf("unknown value type %d", valueType)
		}
		if err != nil {
			return nil, fmt.Errorf("read binary VDF value %q: %w", key, err)
		}
		entries = append(entries, entry)
	}
}

func readBinaryVDFCString(data []byte, offset *int) (string, error) {
	if *offset >= len(data) {
		return "", fmt.Errorf("unexpected end of data")
	}
	end := bytes.IndexByte(data[*offset:], 0)
	if end < 0 {
		return "", fmt.Errorf("missing string terminator")
	}
	value := string(data[*offset : *offset+end])
	*offset += end + 1
	return value, nil
}

func readBinaryVDFBytes(data []byte, offset *int, size int) ([]byte, error) {
	if size < 0 || *offset+size > len(data) {
		return nil, fmt.Errorf("unexpected end of data")
	}
	value := append([]byte(nil), data[*offset:*offset+size]...)
	*offset += size
	return value, nil
}

func encodeBinaryVDF(entries []binaryVDFEntry) ([]byte, error) {
	var buffer bytes.Buffer
	if err := writeBinaryVDFEntries(&buffer, entries); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeBinaryVDFEntries(buffer *bytes.Buffer, entries []binaryVDFEntry) error {
	for _, entry := range entries {
		if entry.Type == binaryVDFEnd {
			return fmt.Errorf("entry %q uses reserved end type", entry.Key)
		}
		buffer.WriteByte(entry.Type)
		writeBinaryVDFCString(buffer, entry.Key)
		switch entry.Type {
		case binaryVDFObject:
			if err := writeBinaryVDFEntries(buffer, entry.Children); err != nil {
				return err
			}
		case binaryVDFString:
			writeBinaryVDFCString(buffer, entry.String)
		case binaryVDFInt32, binaryVDFFloat32, binaryVDFPointer, binaryVDFColor:
			if len(entry.Raw) != 4 {
				return fmt.Errorf("entry %q requires a 4-byte value", entry.Key)
			}
			buffer.Write(entry.Raw)
		case binaryVDFUint64:
			if len(entry.Raw) != 8 {
				return fmt.Errorf("entry %q requires an 8-byte value", entry.Key)
			}
			buffer.Write(entry.Raw)
		case binaryVDFWideString:
			return fmt.Errorf("entry %q uses an unsupported wide string", entry.Key)
		default:
			return fmt.Errorf("entry %q uses unknown value type %d", entry.Key, entry.Type)
		}
	}
	buffer.WriteByte(binaryVDFEnd)
	return nil
}

func writeBinaryVDFCString(buffer *bytes.Buffer, value string) {
	buffer.WriteString(value)
	buffer.WriteByte(0)
}

func binaryVDFIntEntry(key string, value uint32) binaryVDFEntry {
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw, value)
	return binaryVDFEntry{Type: binaryVDFInt32, Key: key, Raw: raw}
}

func binaryVDFStringEntry(key string, value string) binaryVDFEntry {
	return binaryVDFEntry{Type: binaryVDFString, Key: key, String: value}
}

func binaryVDFObjectEntry(key string, children []binaryVDFEntry) binaryVDFEntry {
	return binaryVDFEntry{Type: binaryVDFObject, Key: key, Children: children}
}

func binaryVDFEntryByKey(entries []binaryVDFEntry, key string) *binaryVDFEntry {
	for index := range entries {
		if strings.EqualFold(entries[index].Key, key) {
			return &entries[index]
		}
	}
	return nil
}

func steamShortcutContainer(entries []binaryVDFEntry) *binaryVDFEntry {
	entry := binaryVDFEntryByKey(entries, "shortcuts")
	if entry == nil || entry.Type != binaryVDFObject {
		return nil
	}
	return entry
}

func steamShortcutAppID(entry binaryVDFEntry) (uint32, bool) {
	appID := binaryVDFEntryByKey(entry.Children, "appid")
	if appID == nil || appID.Type != binaryVDFInt32 || len(appID.Raw) != 4 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(appID.Raw), true
}

func steamShortcutExe(entry binaryVDFEntry) string {
	exe := binaryVDFEntryByKey(entry.Children, "Exe")
	if exe == nil || exe.Type != binaryVDFString {
		return ""
	}
	return exe.String
}

func normalizeSteamShortcutExe(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, `"`) {
		if closingQuote := strings.Index(value[1:], `"`); closingQuote >= 0 {
			value = value[1 : closingQuote+1]
		}
	} else {
		value = strings.Trim(value, `"`)
	}
	if value == "" {
		return ""
	}
	absolute, err := filepath.Abs(filepath.Clean(value))
	if err == nil {
		value = absolute
	}
	return strings.ToLower(value)
}

func steamShortcutLongID(appID uint32) string {
	return strconv.FormatUint(uint64(appID)<<32|0x02000000, 10)
}

func steamShortcutAppIDFromLongID(value string) (uint32, bool) {
	longID, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || longID <= uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(longID >> 32), true
}

func findSteamShortcut(entries []binaryVDFEntry, executable string, launchID string) (uint32, bool) {
	appID, _, found := findSteamShortcutEntry(entries, executable, launchID)
	return appID, found
}

func findSteamShortcutEntry(entries []binaryVDFEntry, executable string, launchID string) (uint32, *binaryVDFEntry, bool) {
	container := steamShortcutContainer(entries)
	if container == nil {
		return 0, nil, false
	}
	expectedAppID, hasExpectedAppID := steamShortcutAppIDFromLongID(launchID)
	normalizedExecutable := normalizeSteamShortcutExe(executable)
	for index := range container.Children {
		shortcut := &container.Children[index]
		if shortcut.Type != binaryVDFObject {
			continue
		}
		appID, ok := steamShortcutAppID(*shortcut)
		if !ok {
			continue
		}
		normalizedShortcutExe := normalizeSteamShortcutExe(steamShortcutExe(*shortcut))
		if hasExpectedAppID && appID == expectedAppID &&
			normalizedShortcutExe == normalizedExecutable {
			return appID, shortcut, true
		}
		if normalizedExecutable != "" &&
			normalizedShortcutExe == normalizedExecutable {
			return appID, shortcut, true
		}
	}
	return 0, nil, false
}

func setSteamShortcutLaunchOptions(entries []binaryVDFEntry, executable string, launchID string, launchOptions string) (uint32, bool) {
	appID, shortcut, found := findSteamShortcutEntry(entries, executable, launchID)
	if !found || shortcut == nil {
		return 0, false
	}
	setBinaryVDFStringChild(shortcut, "LaunchOptions", sanitizeSteamShortcutString(launchOptions))
	return appID, true
}

func setBinaryVDFStringChild(parent *binaryVDFEntry, key string, value string) {
	if parent == nil {
		return
	}
	entry := binaryVDFEntryByKey(parent.Children, key)
	if entry != nil && entry.Type == binaryVDFString {
		entry.String = value
		return
	}
	parent.Children = append(parent.Children, binaryVDFStringEntry(key, value))
}

func appendSteamShortcut(entries []binaryVDFEntry, name string, executable string, launchOptions string) ([]binaryVDFEntry, uint32, error) {
	container := steamShortcutContainer(entries)
	if container == nil {
		if len(entries) != 0 {
			return nil, 0, fmt.Errorf("binary VDF has no shortcuts object")
		}
		entries = []binaryVDFEntry{binaryVDFObjectEntry("shortcuts", nil)}
		container = &entries[0]
	}

	name = sanitizeSteamShortcutString(name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
	}
	launchOptions = sanitizeSteamShortcutString(launchOptions)
	quotedExecutable := `"` + executable + `"`
	appID := crc32.ChecksumIEEE([]byte(quotedExecutable+name)) | 0x80000000
	usedAppIDs := make(map[uint32]bool)
	nextIndex := 0
	for _, shortcut := range container.Children {
		if shortcut.Type != binaryVDFObject {
			continue
		}
		if existingAppID, ok := steamShortcutAppID(shortcut); ok {
			usedAppIDs[existingAppID] = true
		}
		if index, err := strconv.Atoi(shortcut.Key); err == nil && index >= nextIndex {
			nextIndex = index + 1
		}
	}
	for usedAppIDs[appID] {
		appID++
		appID |= 0x80000000
	}

	startDirectory := filepath.Dir(executable)
	shortcut := binaryVDFObjectEntry(strconv.Itoa(nextIndex), []binaryVDFEntry{
		binaryVDFIntEntry("appid", appID),
		binaryVDFStringEntry("AppName", name),
		binaryVDFStringEntry("Exe", quotedExecutable),
		binaryVDFStringEntry("StartDir", `"`+startDirectory+`"`),
		binaryVDFStringEntry("icon", ""),
		binaryVDFStringEntry("ShortcutPath", ""),
		binaryVDFStringEntry("LaunchOptions", launchOptions),
		binaryVDFIntEntry("IsHidden", 0),
		binaryVDFIntEntry("AllowDesktopConfig", 1),
		binaryVDFIntEntry("AllowOverlay", 1),
		binaryVDFIntEntry("OpenVR", 0),
		binaryVDFIntEntry("Devkit", 0),
		binaryVDFStringEntry("DevkitGameID", ""),
		binaryVDFIntEntry("DevkitOverrideAppID", 0),
		binaryVDFIntEntry("LastPlayTime", 0),
		binaryVDFStringEntry("FlatpakAppID", ""),
		binaryVDFStringEntry("sortas", ""),
		binaryVDFObjectEntry("tags", []binaryVDFEntry{
			binaryVDFStringEntry("0", "LunaBox"),
		}),
	})
	container.Children = append(container.Children, shortcut)
	return entries, appID, nil
}

func sanitizeSteamShortcutString(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
}
