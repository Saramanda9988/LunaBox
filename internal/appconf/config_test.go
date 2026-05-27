package appconf

import (
	"reflect"
	"testing"
)

func TestNormalizeMetadataSourcesAcceptsOptInSources(t *testing.T) {
	got := normalizeMetadataSources([]string{"bangumi", "dlsite", "erogamescape", "DLSITE", "unknown"})
	want := []string{"bangumi", "dlsite", "erogamescape"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestNormalizeMetadataSourcesDefaultsDoNotIncludeOptInSources(t *testing.T) {
	got := normalizeMetadataSources(nil)

	for _, source := range got {
		if source == "dlsite" || source == "erogamescape" {
			t.Fatalf("opt-in source %q should not be enabled by default: %#v", source, got)
		}
	}
}
