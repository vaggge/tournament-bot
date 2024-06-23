package web

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"tournament-bot/internal/bot"
)

func StartServer(addr string) {
	r := mux.NewRouter()
	//r.HandleFunc("/api/tournaments", getTournaments).Methods("GET")
	//r.HandleFunc("/api/tournaments/{id}", getTournament).Methods("GET")
	//r.HandleFunc("/api/tournaments/{id}/standings", getTournamentStandings).Methods("GET")

	// Добавьте новый маршрут для обработки входящих запросов от Telegram
	r.HandleFunc("/webhook", bot.WebhookHandler).Methods("POST")

	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
