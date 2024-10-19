package main

import (
	"log"
	"net/http"

	"github.com/rishavmehra/gomeet/chat"
)

func serverHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "not found", http.StatusNotFound)
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method is not allowed", http.StatusMethodNotAllowed)
	}
	http.ServeFile(w, r, "home.html")
}

func main() {
	hub := chat.NewHub()
	hub.Run()

	http.HandleFunc("/", serverHome)
	http.ListenAndServe(":8080", nil)
}
