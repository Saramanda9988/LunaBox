package test

import (
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service"
)

func addGameViaMetadata(gameService *service.GameService, game models.Game) error {
	source := game.SourceType
	if source == "" {
		source = enums.Local
	}

	return gameService.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{
		Source: source,
		Game:   game,
	})
}
