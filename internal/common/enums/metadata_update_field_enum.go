package enums

type MetadataUpdateField string

const (
	MetadataUpdateFieldName        MetadataUpdateField = "name"
	MetadataUpdateFieldAliases     MetadataUpdateField = "aliases"
	MetadataUpdateFieldCover       MetadataUpdateField = "cover"
	MetadataUpdateFieldCompany     MetadataUpdateField = "company"
	MetadataUpdateFieldSummary     MetadataUpdateField = "summary"
	MetadataUpdateFieldRating      MetadataUpdateField = "rating"
	MetadataUpdateFieldReleaseDate MetadataUpdateField = "release_date"
	MetadataUpdateFieldTags        MetadataUpdateField = "tags"
)

var DefaultMetadataUpdateFields = []MetadataUpdateField{
	MetadataUpdateFieldName,
	MetadataUpdateFieldAliases,
	MetadataUpdateFieldCover,
	MetadataUpdateFieldCompany,
	MetadataUpdateFieldSummary,
	MetadataUpdateFieldRating,
	MetadataUpdateFieldReleaseDate,
	MetadataUpdateFieldTags,
}

var AllMetadataUpdateFields = []struct {
	Value  MetadataUpdateField
	TSName string
}{
	{MetadataUpdateFieldName, "NAME"},
	{MetadataUpdateFieldAliases, "ALIASES"},
	{MetadataUpdateFieldCover, "COVER"},
	{MetadataUpdateFieldCompany, "COMPANY"},
	{MetadataUpdateFieldSummary, "SUMMARY"},
	{MetadataUpdateFieldRating, "RATING"},
	{MetadataUpdateFieldReleaseDate, "RELEASE_DATE"},
	{MetadataUpdateFieldTags, "TAGS"},
}
