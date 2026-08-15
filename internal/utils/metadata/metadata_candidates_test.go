package metadata

import (
	"reflect"
	"testing"
)

func TestExactMetadataCandidateIndexesReturnsAllMatchesWithinFirstFive(t *testing.T) {
	names := [][]string{
		{"Different Game"},
		{"CLANNAD"},
		{"Clannad "},
		{"Another Game"},
		{"クラナド", "CLANNAD"},
		{"CLANNAD"},
	}

	got := exactMetadataCandidateIndexes("  clannad ", names)
	want := []int{1, 2, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected candidate indexes: got %v, want %v", got, want)
	}
}

func TestExactMetadataCandidateIndexesReturnsEmptyWithoutExactMatch(t *testing.T) {
	got := exactMetadataCandidateIndexes("CLANNAD", [][]string{{"CLANNAD Side Stories"}})
	if len(got) != 0 {
		t.Fatalf("expected no exact candidates, got %v", got)
	}
}
