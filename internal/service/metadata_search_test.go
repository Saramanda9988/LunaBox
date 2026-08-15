package service

import (
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/utils/metadata"
	"testing"
)

func TestFetchImportMetadataSourceReturnsEverySameNameCandidate(t *testing.T) {
	source := metadataSearchSource{
		source: enums.VNDB,
		fetchCandidatesByName: func(string) ([]metadata.MetadataResult, error) {
			return []metadata.MetadataResult{
				{Game: models.Game{Name: "同名游戏", SourceID: "v1"}},
				{Game: models.Game{Name: "同名游戏", SourceID: "v2"}},
			}, nil
		},
	}

	matches, sourceErr := fetchImportMetadataSource(source, "同名游戏")
	if sourceErr != nil {
		t.Fatalf("unexpected source error: %v", sourceErr)
	}
	if len(matches) != 2 {
		t.Fatalf("expected two candidates, got %d", len(matches))
	}
	if matches[0].Game.SourceID != "v1" || matches[1].Game.SourceID != "v2" {
		t.Fatalf("unexpected candidate IDs: %q, %q", matches[0].Game.SourceID, matches[1].Game.SourceID)
	}
}

func TestNormalizedImportMetadataNameSimilarityIgnoresCaseAndSymbols(t *testing.T) {
	if score := normalizedImportMetadataNameSimilarity("SEQUEL blight", "sequel-blight!!"); score != 1 {
		t.Fatalf("expected exact normalized match, got %f", score)
	}
	if score := normalizedImportMetadataNameSimilarity("SEQUEL blight", "VenusBlood HOLLOW"); score >= importMetadataNameSimilarityThreshold {
		t.Fatalf("expected unrelated title below threshold, got %f", score)
	}
}

func TestSelectBestImportMetadataMatchUsesAliases(t *testing.T) {
	matches := []vo.GameMetadataFromWebVO{
		{
			Source: enums.Bangumi,
			Game: models.Game{
				Name:     "调零余波",
				Aliases:  []string{"SEQUEL blight"},
				SourceID: "bgm-1",
			},
		},
		{
			Source: enums.Bangumi,
			Game: models.Game{
				Name:     "VenusBlood HOLLOW",
				SourceID: "bgm-2",
			},
		},
	}

	selected, ok := selectBestImportMetadataMatch(matches, []string{"SEQUEL awake"}, importMetadataNameSimilarityThreshold)
	if ok {
		t.Fatalf("unexpected match for a materially different normalized title: %+v", selected.Game)
	}

	selected, ok = selectBestImportMetadataMatch(matches, []string{"SEQUEL-blight"}, importMetadataNameSimilarityThreshold)
	if !ok || selected.Game.SourceID != "bgm-1" {
		t.Fatalf("expected alias match bgm-1, got ok=%v game=%+v", ok, selected.Game)
	}
}

func TestAttachImportMetadataSourcesAddsEverySelectedProvider(t *testing.T) {
	matches := []vo.GameMetadataFromWebVO{
		{Source: enums.Bangumi, Game: models.Game{Name: "Game", SourceID: "101"}},
		{Source: enums.VNDB, Game: models.Game{Name: "Game", SourceID: "v202"}},
		{Source: enums.Steam, Game: models.Game{Name: "Game"}},
	}

	attached := attachImportMetadataSources(matches)
	for _, match := range attached {
		if len(match.Game.MetadataSources) != 2 {
			t.Fatalf("expected two valid metadata sources, got %+v", match.Game.MetadataSources)
		}
		if match.Game.MetadataSources[0].SourceType != enums.Bangumi || match.Game.MetadataSources[0].SourceID != "101" {
			t.Fatalf("unexpected first metadata source: %+v", match.Game.MetadataSources[0])
		}
		if match.Game.MetadataSources[1].SourceType != enums.VNDB || match.Game.MetadataSources[1].SourceID != "v202" {
			t.Fatalf("unexpected second metadata source: %+v", match.Game.MetadataSources[1])
		}
	}
}
