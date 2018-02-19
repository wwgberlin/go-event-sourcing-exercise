package handlers

import (
	"github.com/wwgberlin/go-event-sourcing-exercise/chess"
	"github.com/wwgberlin/go-event-sourcing-exercise/db"
)

const (
	EventNone int = iota
	EventGameCreated
	EventMoveRequest
	EventMoveSuccess
	EventMoveFail
	EventPromotionRequest
	EventPromotionSuccess
	EventPromotionFail
	EventWhiteWins
	EventBlackWins
	EventDraw
)

func BuildGame(eventStore *db.EventStore, gameID string) *chess.Game {
	return ReplayGame(eventStore, gameID, -1)
}

func ReplayGame(eventStore *db.EventStore, gameID string, lastEventID int) *chess.Game {
	var game *chess.Game

	for _, event := range eventStore.GetEvents() {
		if event.AggregateID != gameID {
			continue
		}

		switch event.EventType {
		case EventGameCreated:
			game = chess.NewGame()
		case EventMoveSuccess:
			game.Move(chess.ParseMove(event.EventData))
		case EventPromotionSuccess:
			game.Promote(chess.ParsePromotion(event.EventData))
		}

		if event.Id == lastEventID {
			return game
		}
	}
	return game
}

func MoveHandler(eventStore *db.EventStore, event db.Event) {
	if event.EventType != EventMoveRequest {
		return
	}
	game := BuildGame(eventStore, event.AggregateID)

	ev := db.Event{
		AggregateID: event.AggregateID,
	}
	if err := game.Move(chess.ParseMove(event.EventData)); err != nil {
		ev.EventType = EventMoveFail
		ev.EventData = err.Error()
	} else {
		ev.EventType = EventMoveSuccess
		ev.EventData = event.EventData
	}
	eventStore.Persist(ev)
}

func PromotionHandler(eventStore *db.EventStore, event db.Event) {
	if event.EventType != EventPromotionRequest {
		return
	}
	game := BuildGame(eventStore, event.AggregateID)

	ev := db.Event{
		AggregateID: event.AggregateID,
	}
	if err := game.Promote(chess.ParsePromotion(event.EventData)); err != nil {
		ev.EventType = EventPromotionFail
		ev.EventData = err.Error()
	} else {
		ev.EventType = EventPromotionSuccess
		ev.EventData = event.EventData
	}
	eventStore.Persist(ev)
}

func StatusChangeHandler(eventStore *db.EventStore, event db.Event) {
	game := BuildGame(eventStore, event.AggregateID)
	status := game.Status()
	if status == 0 {
		return
	}

	ev := db.Event{
		AggregateID: event.AggregateID,
		EventData:   event.EventData,
	}
	if status == 1 {
		ev.EventType = EventWhiteWins
	} else if status == 2 {
		ev.EventType = EventBlackWins
	} else if status == 3 {
		ev.EventType = EventDraw
	}
	eventStore.Persist(ev)
}
