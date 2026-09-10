package room

import (
	"github.com/faytranevozter/7spade/services/ws/internal/protocol"
	"github.com/faytranevozter/7spade/services/ws/internal/session"
)

type clientMessage = protocol.ClientMessage
type playerDelta = protocol.PlayerDelta
type roomSettings = protocol.RoomSettings
type savedCard = protocol.SavedCard
type savedGamePlayer = protocol.SavedGamePlayer
type savedGameResult = protocol.SavedGameResult
type savedReplayMove = protocol.SavedReplayMove
type savedRevealedCard = protocol.SavedRevealedCard
type skinGrant = protocol.SkinGrant

const (
	controlNewGameStarts   = "new_game_starts"
	controlSpectatorAccess = "spectator_access"
	controlEmotes          = "emotes"
)

type tokenClaims = session.Claims

var startAccessCheck = session.StartAccessCheck
