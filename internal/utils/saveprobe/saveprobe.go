package saveprobe

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrUnsupported = errors.New("save path probing is only available on Windows")

type Candidate struct {
	Path          string   `json:"path"`
	Kind          string   `json:"kind"`
	Confidence    string   `json:"confidence"`
	Score         int      `json:"score"`
	WriteCount    int      `json:"write_count"`
	ModifiedFiles int      `json:"modified_files"`
	Signals       []string `json:"signals"`
	LastModified  string   `json:"last_modified,omitempty"`
	Size          int64    `json:"size,omitempty"`
}

type Result struct {
	Candidates     []Candidate `json:"candidates"`
	ObservedEvents int         `json:"observed_events"`
	StartedAt      string      `json:"started_at"`
	EndedAt        string      `json:"ended_at"`
}

type fileActivity struct {
	path             string
	writes           int
	creates          int
	renames          int
	flushes          int
	firstSeen        time.Time
	lastSeen         time.Time
	directoryChanges int
}

type activityStore struct {
	mu             sync.Mutex
	files          map[string]*fileActivity
	observedEvents int
}

func newActivityStore() *activityStore {
	return &activityStore{files: make(map[string]*fileActivity)}
}

func (s *activityStore) recordDirectoryChange(path string, eventID uint16, at time.Time) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || !filepath.IsAbs(path) {
		return
	}

	key := strings.ToLower(path)
	s.mu.Lock()
	defer s.mu.Unlock()

	activity := s.files[key]
	if activity == nil {
		activity = &fileActivity{
			path:      path,
			firstSeen: at,
		}
		s.files[key] = activity
	}
	activity.lastSeen = at
	activity.directoryChanges++
	s.observedEvents++
	switch eventID {
	case 12, 30:
		activity.creates++
	case 16:
		activity.writes++
	case 21:
		activity.flushes++
	case 27:
		activity.renames++
	}
}

func (s *activityStore) result(startedAt time.Time, endedAt time.Time, gameDirectory string) Result {
	s.mu.Lock()
	files := make([]fileActivity, 0, len(s.files))
	for _, activity := range s.files {
		copyActivity := *activity
		files = append(files, copyActivity)
	}
	observedEvents := s.observedEvents
	s.mu.Unlock()

	return Result{
		Candidates:     rankCandidates(files, gameDirectory, endedAt),
		ObservedEvents: observedEvents,
		StartedAt:      startedAt.Format(time.RFC3339),
		EndedAt:        endedAt.Format(time.RFC3339),
	}
}

type directoryActivity struct {
	path       string
	files      int
	writes     int
	maxScore   int
	lastSeen   time.Time
	signalSet  map[string]struct{}
	activities []fileActivity
}

func rankCandidates(files []fileActivity, gameDirectory string, endedAt time.Time) []Candidate {
	directories := make(map[string]*directoryActivity)
	fileCandidates := make([]Candidate, 0, len(files))

	for _, activity := range files {
		candidate, ok := scoreFileCandidate(activity, gameDirectory, endedAt)
		if !ok {
			continue
		}
		fileCandidates = append(fileCandidates, candidate)

		directory := filepath.Dir(activity.path)
		key := strings.ToLower(directory)
		group := directories[key]
		if group == nil {
			group = &directoryActivity{
				path:      directory,
				signalSet: make(map[string]struct{}),
			}
			directories[key] = group
		}
		group.files++
		group.writes += activity.writes
		group.maxScore = max(group.maxScore, candidate.Score)
		if activity.lastSeen.After(group.lastSeen) {
			group.lastSeen = activity.lastSeen
		}
		group.activities = append(group.activities, activity)
		for _, signal := range candidate.Signals {
			group.signalSet[signal] = struct{}{}
		}
	}

	directoryCandidates := make([]Candidate, 0, len(directories))
	for _, group := range directories {
		score := group.maxScore + min(18, group.files*4) + min(10, group.writes)
		signals := signalSlice(group.signalSet)
		if group.files >= 2 {
			signals = appendUnique(signals, "related_files")
		}
		directoryCandidates = append(directoryCandidates, Candidate{
			Path:          group.path,
			Kind:          "directory",
			Confidence:    confidenceForScore(score),
			Score:         min(score, 100),
			WriteCount:    group.writes,
			ModifiedFiles: group.files,
			Signals:       signals,
			LastModified:  group.lastSeen.Format(time.RFC3339),
		})
	}

	all := append(directoryCandidates, fileCandidates...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].Score == all[j].Score {
			if all[i].Kind != all[j].Kind {
				return all[i].Kind == "directory"
			}
			return strings.ToLower(all[i].Path) < strings.ToLower(all[j].Path)
		}
		return all[i].Score > all[j].Score
	})

	if len(all) > 20 {
		all = all[:20]
	}
	return all
}

func scoreFileCandidate(activity fileActivity, gameDirectory string, endedAt time.Time) (Candidate, bool) {
	info, err := os.Stat(activity.path)
	if err != nil || info.IsDir() {
		return Candidate{}, false
	}

	meaningfulWrites := activity.writes + activity.creates + activity.renames
	if meaningfulWrites == 0 {
		return Candidate{}, false
	}

	modifiedDuringProbe := !info.ModTime().Before(activity.firstSeen.Add(-3 * time.Second))
	if activity.writes == 0 && activity.renames == 0 && !modifiedDuringProbe {
		return Candidate{}, false
	}

	score := 20 + min(25, activity.writes*5) + min(12, activity.creates*4) + min(8, activity.flushes*2)
	signals := make([]string, 0, 6)
	if activity.writes >= 2 {
		signals = append(signals, "frequent_writes")
	}
	if activity.directoryChanges > 0 {
		signals = append(signals, "directory_change")
	}
	if modifiedDuringProbe && !info.ModTime().After(endedAt.Add(3*time.Second)) {
		score += 18
		signals = append(signals, "recent_change")
	}

	extension := strings.ToLower(filepath.Ext(activity.path))
	if isLikelySaveExtension(extension) {
		score += 18
		signals = append(signals, "save_like_extension")
	}
	if info.Size() > 0 && info.Size() <= 128*1024*1024 {
		score += 6
	}

	lowerPath := strings.ToLower(activity.path)
	if isUserDataPath(lowerPath) {
		score += 12
		signals = append(signals, "user_data_location")
	}
	if isUnderDirectory(activity.path, gameDirectory) {
		score += 8
		signals = append(signals, "game_directory")
	}
	if isLikelyIncidentalPath(lowerPath, extension) {
		score -= 32
		signals = append(signals, "incidental_activity")
	}

	score = int(math.Max(0, math.Min(100, float64(score))))
	return Candidate{
		Path:          activity.path,
		Kind:          "file",
		Confidence:    confidenceForScore(score),
		Score:         score,
		WriteCount:    activity.writes,
		ModifiedFiles: 1,
		Signals:       signals,
		LastModified:  info.ModTime().Format(time.RFC3339),
		Size:          info.Size(),
	}, true
}

func confidenceForScore(score int) string {
	switch {
	case score >= 72:
		return "high"
	case score >= 48:
		return "medium"
	default:
		return "low"
	}
}

func isLikelySaveExtension(extension string) bool {
	switch extension {
	case ".sav", ".save", ".sud", ".dat", ".bin", ".db", ".sqlite", ".json", ".xml", ".ini", ".cfg", ".rpgsave", ".rvdata", ".rvdata2", ".rxdata":
		return true
	default:
		return false
	}
}

func isUserDataPath(lowerPath string) bool {
	markers := []string{"\\appdata\\", "\\documents\\", "\\saved games\\", "\\save games\\", "\\userdata\\", "\\steam\\userdata\\"}
	for _, marker := range markers {
		if strings.Contains(lowerPath, marker) {
			return true
		}
	}
	return false
}

func isLikelyIncidentalPath(lowerPath string, extension string) bool {
	markers := []string{"\\cache\\", "\\temp\\", "\\tmp\\", "\\logs\\", "\\log\\", "shadercache", "crashpad", "webcache", "gpu cache"}
	for _, marker := range markers {
		if strings.Contains(lowerPath, marker) {
			return true
		}
	}
	switch extension {
	case ".log", ".tmp", ".dmp", ".dll", ".exe", ".pdb", ".cache":
		return true
	default:
		return false
	}
}

func isUnderDirectory(path string, directory string) bool {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absDirectory, err := filepath.Abs(directory)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(absDirectory, absPath)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func signalSlice(signalSet map[string]struct{}) []string {
	result := make([]string, 0, len(signalSet))
	for signal := range signalSet {
		result = append(result, signal)
	}
	sort.Strings(result)
	return result
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}
