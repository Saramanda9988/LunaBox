package gamehelper

import (
	"encoding/json"
	"fmt"
	"strings"
)

func NormalizeAliases(aliases []string) []string {
	normalized := make([]string, 0, len(aliases))
	seen := make(map[string]struct{}, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		key := strings.ToLower(alias)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, alias)
	}
	return normalized
}

func MergeAliases(existing []string, scraped []string) []string {
	merged := make([]string, 0, len(existing)+len(scraped))
	merged = append(merged, existing...)
	merged = append(merged, scraped...)
	return NormalizeAliases(merged)
}

func EncodeAliases(aliases []string) string {
	encoded, err := json.Marshal(NormalizeAliases(aliases))
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func DecodeAliases(encoded string) ([]string, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return []string{}, nil
	}
	var aliases []string
	if err := json.Unmarshal([]byte(encoded), &aliases); err != nil {
		return nil, fmt.Errorf("decode game aliases: %w", err)
	}
	return NormalizeAliases(aliases), nil
}
