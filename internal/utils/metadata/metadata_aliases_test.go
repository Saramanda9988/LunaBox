package metadata

import (
	"reflect"
	"testing"
)

func TestHikarinagiMetadataAliases(t *testing.T) {
	translatedTitle := "克兰娜德"
	result := NewHikarinagiInfoGetter().convertToMetadataResult(hikarinagiGame{
		ID:          371,
		OriginTitle: " CLANNAD ",
		TransTitle:  &translatedTitle,
		Aliases:     []string{"クラナド", "clannad", "克兰娜德"},
	})
	want := []string{"CLANNAD", "クラナド"}
	if !reflect.DeepEqual(result.Game.Aliases, want) {
		t.Fatalf("Hikarinagi aliases: got %#v want %#v", result.Game.Aliases, want)
	}
}

func TestTouchGalMetadataAliases(t *testing.T) {
	result := NewTouchGalInfoGetter().convertToMetadataResult(touchGalGameData{
		UniqueID: "abcd1234",
		Name:     "Summer Pockets",
		Aliases:  []string{"サマポケ", " summer pockets ", "サマポケ"},
	})
	want := []string{"サマポケ"}
	if !reflect.DeepEqual(result.Game.Aliases, want) {
		t.Fatalf("TouchGAL aliases: got %#v want %#v", result.Game.Aliases, want)
	}
}

func TestVNDBMetadataAliases(t *testing.T) {
	getter := VNDBInfoGetter{preferredLangs: []string{"zh-hans"}}
	game := getter.convertResultToGame(vndbQueryResult{
		ID:      "v1",
		Title:   "Subarashiki Hibi",
		Aliases: []string{"SubaHibi", "素晴らしき日々～不連続存在～"},
		Titles: []vndbTitle{
			{Lang: "ja", Title: "素晴らしき日々～不連続存在～", Latin: "Subarashiki Hibi", Main: true},
			{Lang: "zh-hans", Title: "美好的每一天", Official: true},
		},
	})
	want := []string{"Subarashiki Hibi", "素晴らしき日々～不連続存在～", "SubaHibi"}
	if !reflect.DeepEqual(game.Aliases, want) {
		t.Fatalf("VNDB aliases: got %#v want %#v", game.Aliases, want)
	}
}

func TestBangumiMetadataAliases(t *testing.T) {
	subject := bangumiResponse{
		Name:   "CLANNAD -クラナド-",
		NameCN: "CLANNAD",
		Infobox: []bangumiInfoboxItem{
			{Key: "开发商", Value: "Key"},
			{Key: "别名", Value: []interface{}{
				map[string]interface{}{"v": "クラナド"},
				map[string]interface{}{"v": "小镇家族"},
				map[string]interface{}{"v": "clannad"},
			}},
		},
	}
	want := []string{"CLANNAD -クラナド-", "クラナド", "小镇家族"}
	got := buildBangumiAliases(subject, subject.NameCN)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Bangumi aliases: got %#v want %#v", got, want)
	}
}
