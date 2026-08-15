package gamehelper

import (
	"reflect"
	"testing"

	"lunabox/internal/common/enums"
)

func TestMergeAliasesKeepsManualAliases(t *testing.T) {
	got := MergeAliases(
		[]string{"手动简称", "SubaHibi"},
		[]string{"subahibi", "素晴らしき日々～不連続存在～"},
	)
	want := []string{"手动简称", "SubaHibi", "素晴らしき日々～不連続存在～"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged aliases: got %#v want %#v", got, want)
	}
}

func TestDefaultMetadataUpdateFieldsIncludeAliases(t *testing.T) {
	fields := NormalizeMetadataUpdateFields(nil)
	if !fields.Has(enums.MetadataUpdateFieldAliases) {
		t.Fatal("default metadata update fields should include aliases")
	}
}
