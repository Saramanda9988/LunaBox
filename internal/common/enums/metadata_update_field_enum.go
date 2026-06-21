package enums

type MetadataUpdateField string

const (
	MetadataUpdateFieldName        MetadataUpdateField = "name"
	MetadataUpdateFieldCover       MetadataUpdateField = "cover"
	MetadataUpdateFieldCompany     MetadataUpdateField = "company"
	MetadataUpdateFieldSummary     MetadataUpdateField = "summary"
	MetadataUpdateFieldRating      MetadataUpdateField = "rating"
	MetadataUpdateFieldReleaseDate MetadataUpdateField = "release_date"
	MetadataUpdateFieldTags        MetadataUpdateField = "tags"
)

var DefaultMetadataUpdateFields = []MetadataUpdateField{
	MetadataUpdateFieldName,
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
	{MetadataUpdateFieldCover, "COVER"},
	{MetadataUpdateFieldCompany, "COMPANY"},
	{MetadataUpdateFieldSummary, "SUMMARY"},
	{MetadataUpdateFieldRating, "RATING"},
	{MetadataUpdateFieldReleaseDate, "RELEASE_DATE"},
	{MetadataUpdateFieldTags, "TAGS"},
}
