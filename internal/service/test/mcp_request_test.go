package test

import (
	"encoding/json"
	"testing"

	"lunabox/internal/common/vo"
)

func TestMCPGetGameRequestAcceptsStringOrNumberGameID(t *testing.T) {
	var stringReq vo.MCPGetGameRequest
	if err := json.Unmarshal([]byte(`{"game_id":"game-1"}`), &stringReq); err != nil {
		t.Fatalf("string game_id unmarshal failed: %v", err)
	}
	if string(stringReq.GameID) != "game-1" {
		t.Fatalf("unexpected string game_id: %s", string(stringReq.GameID))
	}

	var numberReq vo.MCPGetGameRequest
	if err := json.Unmarshal([]byte(`{"game_id":1}`), &numberReq); err != nil {
		t.Fatalf("number game_id unmarshal failed: %v", err)
	}
	if string(numberReq.GameID) != "1" {
		t.Fatalf("unexpected numeric game_id: %s", string(numberReq.GameID))
	}
}
