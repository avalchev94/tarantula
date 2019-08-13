package tarantula

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"

	"github.com/avalchev94/tarantula/games/connect4"
	"github.com/gorilla/websocket"
)

var (
	rooms    = NewRooms()
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	}
}

func handleListRooms(w http.ResponseWriter, r *http.Request) {
	roomsData := []interface{}{}
	// for name, r := range rooms {
	// 	data := struct {
	// 		Name    string
	// 		Players int
	// 		Game    string
	// 	}{name, len(r.Players), r.Game.Name()}

	// 	roomsData = append(roomsData, data)
	// }

	if err := json.NewEncoder(w).Encode(roomsData); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func handleNewRoom(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	err := rooms.Add(name, &Room{
		Mutex:   &sync.Mutex{},
		Game:    connect4.NewGame(7, 6),
		Players: Players{},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Room '%s' was created.", name)
	w.WriteHeader(http.StatusCreated)

	// Wait certain time, if no client joins, delete room
	time.AfterFunc(10*time.Second, func() {
		room, err := rooms.Get(name)
		if err == nil && len(room.Players) == 0 {
			log.Printf("Deleting room '%s'. Room is empty after 10 seconds...", name)
			rooms.Delete(name)
		}
	})
}

func handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	room, err := rooms.Get(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uuidStr := r.URL.Query().Get("uuid")
	if _, err := uuid.FromString(uuidStr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := room.AddPlayer(uuidStr, conn); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO: start the game on user's request
	if len(room.Players) == 2 {
		go func() {
			log.Printf("Room '%s' is starting...", name)
			if err := room.Run(); err != nil {
				log.Fatalf("Room '%s' failed: %s", name, err)
			}
		}()
	}
}
