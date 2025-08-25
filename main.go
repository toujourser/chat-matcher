package main

import (
	"log"
	"net/http"

	"github.com/toujourser/chat-matcher/handler"
)

func main() {
	http.HandleFunc("/match", handler.HandleMatch)
	http.HandleFunc("/ws", handler.HandleWS)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
