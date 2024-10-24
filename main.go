package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/rishavmehra/gomeet/chat"
	"github.com/rishavmehra/gomeet/server"
)

var addr = flag.String("addr", ":8080", "http service address")

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
	flag.Parse()
	hub := chat.NewHub()
	go hub.Run()
	go server.Run()

	http.HandleFunc("/", serverHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		chat.WsUpgrader(w, r, hub)
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}
