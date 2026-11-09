package appconf

import (
	"testing"

	enums2 "lunabox/backend/common/enums"
)

func TestNormalizeMetadataStrategyMigratesPreferredSource(t *testing.T) {
	config := &AppConfig{BatchImportPreferredSource: "vndb"}
	if !NormalizeMetadataStrategy(config) {
		t.Fatal("expected change")
	}
	if got := config.Strategy(); got.Base != enums2.VNDB || len(got.Fields) != 0 {
		t.Fatalf("strategy = %+v", got)
	}
	// 迁移只在没有组合策略时发生：之后把基底改成“不指定”不会被旧值覆盖。
	config.SetStrategy(MetadataStrategy{})
	NormalizeMetadataStrategy(config)
	if config.Strategy().Base != "" {
		t.Fatalf("base = %q", config.Strategy().Base)
	}
}

func TestNormalizeMetadataStrategyDropsUnknownValues(t *testing.T) {
	config := &AppConfig{MetadataStrategy: &MetadataStrategy{
		Base: " Bangumi ", Enabled: true,
		Fields: map[enums2.MetadataUpdateField][]enums2.SourceType{
			enums2.MetadataUpdateFieldName:    {"unknown", "VNDB", "vndb"},
			enums2.MetadataUpdateFieldAliases: {"all"},
			enums2.MetadataUpdateFieldSummary: {"all"},
			"no_such_field":                   {"vndb"},
		},
	}}
	NormalizeMetadataStrategy(config)
	got := config.Strategy()
	if got.Base != enums2.Bangumi || !got.Enabled {
		t.Fatalf("base = %q, enabled = %v", got.Base, got.Enabled)
	}
	if len(got.Fields) != 2 || got.FieldSource(enums2.MetadataUpdateFieldName) != enums2.VNDB ||
		len(got.Fields[enums2.MetadataUpdateFieldName]) != 1 ||
		got.FieldSource(enums2.MetadataUpdateFieldAliases) != MetadataStrategyAllSources {
		t.Fatalf("fields = %+v", got.Fields)
	}
}

func TestStrategyCopiesAreIndependent(t *testing.T) {
	config := &AppConfig{}
	config.SetStrategy(MetadataStrategy{Base: enums2.VNDB}.WithFieldSource(enums2.MetadataUpdateFieldName, enums2.Bangumi))
	copyConfig := *config
	copyConfig.SetStrategy(copyConfig.Strategy().WithFieldSource(enums2.MetadataUpdateFieldName, ""))
	if config.Strategy().FieldSource(enums2.MetadataUpdateFieldName) != enums2.Bangumi {
		t.Fatal("changing a config copy modified the original strategy")
	}
}
