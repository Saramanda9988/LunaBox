package test

import (
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/service"
	"lunabox/internal/vo"
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
