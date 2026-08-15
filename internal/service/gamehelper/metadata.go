package gamehelper

import (
	"reflect"
	"strings"

	"lunabox/internal/appconf"
	enums2 "lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils/metadata"
)

func IsEmptyGame(game models.Game) bool {
	if len(game.Aliases) == 0 {
		game.Aliases = nil
	}
	return reflect.DeepEqual(game, models.Game{})
}

// MetadataUpdateFieldSet is the set of metadata fields to refresh from a remote source.
type MetadataUpdateFieldSet map[enums2.MetadataUpdateField]struct{}

// Has reports whether the field is included in the update set.
func (fields MetadataUpdateFieldSet) Has(field enums2.MetadataUpdateField) bool {
	_, ok := fields[field]
	return ok
}

// MetadataGetterOptions returns the shared getter options applied to all metadata sources,
// wiring up proxy and tag-limit configuration.
func MetadataGetterOptions(config *appconf.AppConfig) []metadata.GetterOption {
	if config == nil {
		return nil
	}
	return []metadata.GetterOption{
		metadata.WithProxyConfig(config),
		metadata.WithTagLimit(config.ScrapedTagLimit),
		metadata.WithBangumiCoverSource(config.BangumiCoverSource),
		metadata.WithVNDBCoverSource(config.VNDBCoverSource),
		metadata.WithSteamCoverOrientation(config.SteamCoverOrientation),
	}
}

// ConfiguredMetadataSources returns the enabled metadata sources in user-preferred order,
// falling back to a sensible default when the config is empty or invalid.
func ConfiguredMetadataSources(config *appconf.AppConfig) []enums2.SourceType {
	defaultSources := []enums2.SourceType{enums2.Bangumi, enums2.VNDB, enums2.Ymgal, enums2.Steam}
	if config == nil || len(config.MetadataSources) == 0 {
		return defaultSources
	}

	result := make([]enums2.SourceType, 0, len(config.MetadataSources))
	seen := make(map[enums2.SourceType]struct{}, len(config.MetadataSources))
	for _, source := range config.MetadataSources {
		normalized := NormalizeMetadataSourceType(enums2.SourceType(source))
		switch normalized {
		case enums2.Bangumi, enums2.VNDB, enums2.Ymgal, enums2.Steam, enums2.DLsite, enums2.TouchGal, enums2.Hikarinagi, enums2.ErogameScape:
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
	}

	if len(result) == 0 {
		return defaultSources
	}
	return result
}

// NormalizeMetadataSourceType lowercases and trims a source type for comparison.
func NormalizeMetadataSourceType(source enums2.SourceType) enums2.SourceType {
	return enums2.SourceType(strings.ToLower(strings.TrimSpace(string(source))))
}

// NormalizeMetadataUpdateFields validates the requested fields and falls back to the
// default field set when none are supplied.
func NormalizeMetadataUpdateFields(fields []enums2.MetadataUpdateField) MetadataUpdateFieldSet {
	fieldSet := make(MetadataUpdateFieldSet, len(enums2.DefaultMetadataUpdateFields))
	for _, field := range fields {
		normalized := enums2.MetadataUpdateField(strings.ToLower(strings.TrimSpace(string(field))))
		switch normalized {
		case enums2.MetadataUpdateFieldName,
			enums2.MetadataUpdateFieldAliases,
			enums2.MetadataUpdateFieldCover,
			enums2.MetadataUpdateFieldCompany,
			enums2.MetadataUpdateFieldSummary,
			enums2.MetadataUpdateFieldRating,
			enums2.MetadataUpdateFieldReleaseDate,
			enums2.MetadataUpdateFieldTags:
			fieldSet[normalized] = struct{}{}
		}
	}

	if len(fieldSet) > 0 {
		return fieldSet
	}

	for _, field := range enums2.DefaultMetadataUpdateFields {
		fieldSet[field] = struct{}{}
	}
	return fieldSet
}

// IsVndbID matches the VNDB id format (a "v" prefix followed by digits).
func IsVndbID(sourceID string) bool {
	return strings.HasPrefix(sourceID, "v") && len(sourceID) > 1
}

// IsYmgalID matches the Ymgal id format (a "ga" prefix followed by digits).
func IsYmgalID(sourceID string) bool {
	return strings.HasPrefix(sourceID, "ga") && len(sourceID) > 2
}

// IsSteamAppID accepts raw numeric Steam AppIDs as well as common URL/protocol forms
// like steam://rungameid/620 or store.steampowered.com/app/620/.
func IsSteamAppID(sourceID string) bool {
	id := strings.TrimSpace(sourceID)
	if id == "" {
		return false
	}

	inDigits := false
	for i := 0; i < len(id); i++ {
		if id[i] >= '0' && id[i] <= '9' {
			inDigits = true
			continue
		}
		if inDigits {
			return true
		}
	}
	return inDigits
}

// MetadataRefreshBatchKeyForGame returns the key used to group games sharing the same
// metadata source/id so that one remote fetch can serve all of them.
func MetadataRefreshBatchKeyForGame(game models.Game) string {
	return string(NormalizeMetadataSourceType(game.SourceType)) + ":" + strings.ToLower(strings.TrimSpace(game.SourceID))
}
