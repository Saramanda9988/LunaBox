package appconf

import (
	"strings"
	"time"

	enums2 "lunabox/internal/common/enums"
	"lunabox/internal/utils/proxyutils"
)

func NormalizeScheduledDBBackup(config *AppConfig) bool {
	if config == nil {
		return false
	}

	changed := false
	mode := strings.ToLower(strings.TrimSpace(config.ScheduledDBBackupMode))
	if mode != ScheduledDBBackupModeDaily {
		mode = ScheduledDBBackupModeInterval
	}
	if config.ScheduledDBBackupMode != mode {
		config.ScheduledDBBackupMode = mode
		changed = true
	}

	interval := config.ScheduledDBBackupIntervalMinutes
	if interval < MinScheduledDBBackupIntervalMinutes {
		interval = DefaultScheduledDBBackupIntervalMinutes
	}
	if interval > MaxScheduledDBBackupIntervalMinutes {
		interval = MaxScheduledDBBackupIntervalMinutes
	}
	if config.ScheduledDBBackupIntervalMinutes != interval {
		config.ScheduledDBBackupIntervalMinutes = interval
		changed = true
	}

	backupTime := strings.TrimSpace(config.ScheduledDBBackupTime)
	if _, err := time.Parse("15:04", backupTime); err != nil {
		backupTime = DefaultScheduledDBBackupTime
	}
	if config.ScheduledDBBackupTime != backupTime {
		config.ScheduledDBBackupTime = backupTime
		changed = true
	}

	return changed
}

func NormalizeScrapedTagLimit(limit int) int {
	if limit < -1 {
		return -1
	}
	return limit
}

func NormalizeLocalDBBackupRetention(retention int) int {
	if retention < 1 {
		return DefaultLocalDBBackupRetention
	}
	return retention
}

func NormalizeHomeGameCarouselIntervalSec(intervalSec int) int {
	if intervalSec <= 0 {
		return DefaultHomeGameCarouselIntervalSec
	}
	if intervalSec < MinHomeGameCarouselIntervalSec {
		return MinHomeGameCarouselIntervalSec
	}
	return intervalSec
}

func NormalizeProcessDetectionTimeoutSec(timeoutSec int) int {
	if timeoutSec <= 0 {
		return DefaultProcessDetectionTimeoutSec
	}
	if timeoutSec < MinProcessDetectionTimeoutSec {
		return MinProcessDetectionTimeoutSec
	}
	if timeoutSec > MaxProcessDetectionTimeoutSec {
		return MaxProcessDetectionTimeoutSec
	}
	return timeoutSec
}

func NormalizeBatchImportPreferences(config *AppConfig) bool {
	if config == nil {
		return false
	}

	changed := false
	switch config.BatchImportScanPreset {
	case "scan_parent", "scan_library_child", "hierarchy_child":
	default:
		config.BatchImportScanPreset = DefaultBatchImportScanPreset
		changed = true
	}

	if config.BatchImportHierarchyDepth < 0 {
		config.BatchImportHierarchyDepth = 0
		changed = true
	}
	if config.BatchImportHierarchyDepth > MaxBatchImportHierarchyDepth {
		config.BatchImportHierarchyDepth = MaxBatchImportHierarchyDepth
		changed = true
	}

	if config.BatchImportPreferredSource != "" {
		if _, ok := allowedMetadataSourceSet[config.BatchImportPreferredSource]; !ok {
			config.BatchImportPreferredSource = ""
			changed = true
		}
	}

	return changed
}

func normalizeMetadataSources(sources []string) []string {
	if len(sources) == 0 {
		return cloneStringSlice(defaultMetadataSources)
	}

	result := make([]string, 0, len(defaultMetadataSources))
	seen := make(map[string]struct{}, len(defaultMetadataSources))

	for _, source := range sources {
		normalized := strings.ToLower(strings.TrimSpace(source))
		if normalized == "" {
			continue
		}
		if _, ok := allowedMetadataSourceSet[normalized]; !ok {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}

		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	if len(result) == 0 {
		return cloneStringSlice(defaultMetadataSources)
	}
	return result
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}

func NormalizeMCPPort(port int) int {
	if port < 1 || port > 65535 {
		return DefaultMCPPort
	}
	return port
}

func NormalizeGameCardLayout(layout string) string {
	switch strings.ToLower(strings.TrimSpace(layout)) {
	case "landscape":
		return "landscape"
	default:
		return DefaultGameCardLayout
	}
}

func NormalizeMetadataCoverSource(source enums2.MetadataCoverSource) enums2.MetadataCoverSource {
	switch strings.ToLower(strings.TrimSpace(string(source))) {
	case string(enums2.MetadataCoverSourceOriginal):
		return enums2.MetadataCoverSourceOriginal
	default:
		return enums2.MetadataCoverSourceHikarinagi
	}
}

func NormalizeMetadataCoverSources(config *AppConfig) {
	if config == nil {
		return
	}
	config.BangumiCoverSource = NormalizeMetadataCoverSource(config.BangumiCoverSource)
	config.VNDBCoverSource = NormalizeMetadataCoverSource(config.VNDBCoverSource)
}

func NormalizeProxySettings(config *AppConfig) bool {
	if config == nil {
		return false
	}

	changed := false
	normalizeMode := func(value string) string {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case proxyutils.ProxyModeManual:
			return proxyutils.ProxyModeManual
		case proxyutils.ProxyModeDirect:
			return proxyutils.ProxyModeDirect
		default:
			return proxyutils.ProxyModeSystem
		}
	}
	setMode := func(target *string) {
		next := normalizeMode(*target)
		if *target != next {
			*target = next
			changed = true
		}
	}

	trimmedProxyURL := strings.TrimSpace(config.NetworkProxyURL)
	if config.NetworkProxyURL != trimmedProxyURL {
		config.NetworkProxyURL = trimmedProxyURL
		changed = true
	}

	setMode(&config.NetworkProxyMode)

	return changed
}
