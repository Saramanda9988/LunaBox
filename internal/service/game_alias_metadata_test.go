package service

import (
	"context"
	"reflect"
	"testing"

	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service/gamehelper"
	"lunabox/internal/utils/metadata"
)

func TestApplyRemoteMetadataMergesAliases(t *testing.T) {
	db := setupImportServiceTestDB(t)
	service := NewGameService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	existing := models.Game{
		ID:      "alias-refresh-game",
		Name:    "本地名称",
		Aliases: []string{"手动简称", "SubaHibi"},
	}
	if err := service.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{Game: existing}); err != nil {
		t.Fatalf("add game: %v", err)
	}
	existing, err := service.GetGameByID(existing.ID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}

	fields := gamehelper.NormalizeMetadataUpdateFields([]enums.MetadataUpdateField{
		enums.MetadataUpdateFieldAliases,
	})
	_, err = service.applyRemoteMetadataResult(existing, metadata.MetadataResult{
		Game: models.Game{
			Name:    "远端名称",
			Aliases: []string{"subahibi", "素晴らしき日々～不連続存在～"},
		},
	}, false, fields)
	if err != nil {
		t.Fatalf("apply remote metadata: %v", err)
	}

	saved, err := service.GetGameByID(existing.ID)
	if err != nil {
		t.Fatalf("get updated game: %v", err)
	}
	wantAliases := []string{"手动简称", "SubaHibi", "素晴らしき日々～不連続存在～"}
	if !reflect.DeepEqual(saved.Aliases, wantAliases) {
		t.Fatalf("aliases: got %#v want %#v", saved.Aliases, wantAliases)
	}
	if saved.Name != existing.Name {
		t.Fatalf("unselected name changed: got %q want %q", saved.Name, existing.Name)
	}
}
