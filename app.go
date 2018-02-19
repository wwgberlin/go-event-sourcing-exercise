package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/wwgberlin/go-event-sourcing-exercise/chess"
	"github.com/wwgberlin/go-event-sourcing-exercise/db"
	"github.com/wwgberlin/go-event-sourcing-exercise/handlers"
	"github.com/wwgberlin/go-event-sourcing-exercise/namegen"
	"golang.org/x/net/websocket"
)

type app struct {
	db *db.EventStore
}

type Board struct {
	Squares [][]chess.Square
	Moves   []string
}
type page struct {
	Name  string
	Board Board
}

type msg struct {
	Target      string
	AggregateId string
	Type        string
}

func newApi(d *db.EventStore) *app {
	a := app{db: d}
	a.db.Register(db.NewEventHandler(handlers.MoveHandler))
	a.db.Register(db.NewEventHandler(handlers.PromotionHandler))
	a.db.Register(db.NewEventHandler(handlers.StatusChangeHandler))

	return &a
}

func (a *app) getGame(gameID string) *chess.Game {
	if gameID == "" {
		gameID = namegen.Generate()
		log.Println(fmt.Errorf("New game created: %s", gameID))
	}
	return handlers.BuildGame(a.db, gameID)
}

func (a *app) replayGame(gameID string, lastEventID int) *chess.Game {
	return handlers.ReplayGame(a.db, gameID, lastEventID)
}

func (a *app) newGameHandler(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadFile("./public/static/new_game.html")
	if err != nil {
		log.Println(err)
	}

	w.Write(data)
}

func (a *app) createGameHandler(w http.ResponseWriter, r *http.Request) {
	gameID := namegen.Generate()

	a.db.Persist(db.Event{
		Id:          0,
		AggregateID: gameID,
		EventData:   "",
		EventType:   handlers.EventGameCreated,
	})
	log.Println(fmt.Errorf("New game created: %s", gameID))

	w.Header().Add("Location", "/game?game_id="+gameID)
}

func (a *app) gameHandler(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("game_id")
	game := a.getGame(gameID)

	var tpl bytes.Buffer
	t := template.Must(template.ParseFiles("templates/board.tmpl", "templates/game.tmpl"))
	if err := t.ExecuteTemplate(&tpl, "base", page{
		Name: gameID, Board: Board{Squares: game.Draw(), Moves: game.Moves()}}); err != nil {
		panic(err)
	}
	w.Write(tpl.Bytes())
}

func (a *app) boardHandler(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("game_id")
	game := a.getGame(gameID)

	var tpl bytes.Buffer
	t := template.Must(template.ParseFiles("templates/board.tmpl"))
	if err := t.ExecuteTemplate(&tpl, "board", Board{game.Draw(), game.Moves()}); err != nil {
		panic(err)
	}
	w.Write(tpl.Bytes())
}

// debugHandler writes string representation of current board state to http response
// it doesn't have any information about current game, only a list of moves, from which it builds the state
func (a *app) debugHandler(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("game_id")
	game := a.getGame(gameID)

	if _, err := w.Write([]byte(game.Debug())); err != nil {
		log.Printf("can't write the response: %v", err)
	}
}

func (a *app) promotionsHandler(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("game_id")
	game := a.getGame(gameID)

	query := r.URL.Query().Get("target")
	move := chess.ParseMove(query)
	promotions := game.ValidPromotions(move)
	strs := make([]string, len(promotions))
	for i := range promotions {
		strs[i] = promotions[i].ImagePath()
	}
	w.Write([]byte(strings.Join(strs, ",")))
	return
}

func (a *app) eventIDsHandler(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("game_id")
	var ids []int
	for _, e := range a.db.GetEvents() {
		if e.AggregateID == gameID {
			ids = append(ids, e.Id)
		}
	}
	res, err := json.Marshal(ids)
	if err != nil {
		log.Println(err)
	}
	w.Write(res)
}

func (a *app) pageSubscriber(ws *websocket.Conn, m msg) db.EventHandler {
	return db.NewEventHandler(
		func(eventStore *db.EventStore, e db.Event) {
			if e.AggregateID == m.AggregateId {
				switch e.EventType {
				case handlers.EventMoveSuccess,
					handlers.EventPromotionSuccess:
					ws.Write([]byte("1"))
				case handlers.EventMoveFail,
					handlers.EventPromotionFail:
					ws.Write([]byte("0"))
				}
			}
		})
}
func (a *app) wsHandler(ws *websocket.Conn) {
	var m msg
	if err := websocket.JSON.Receive(ws, &m); err != nil {
		return
	}
	sub := a.pageSubscriber(ws, m)
	a.db.Register(sub)
	for {
		e := db.Event{AggregateID: m.AggregateId, EventData: m.Target}
		switch m.Type {
		case "move":
			e.EventType = handlers.EventMoveRequest
		case "promote":
			e.EventType = handlers.EventPromotionRequest
		default:
			continue
		}
		a.db.Persist(e)
		if err := websocket.JSON.Receive(ws, &m); err != nil {
			a.db.Deregister(sub)
			fmt.Println("closed")
			return
		}
	}
}

func (a *app) scoreHandler(w http.ResponseWriter, r *http.Request) {
	data := handlers.BuildScores(a.db)
	output, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
	}
	w.Write(output)
}