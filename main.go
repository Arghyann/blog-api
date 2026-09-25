package main

import (
	"log"
	"net/http"
)

func main() {
	//connect to db
	db := initDb("./posts.db")
	defer db.Close()
	api := &API{
		db: db,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /posts/{slug}", api.getPost)
	mux.HandleFunc("GET /posts", api.listPosts)
	// 4. Start the HTTP server
	log.Println("Server running on port 8080")

	log.Fatal(http.ListenAndServe(":8080", mux))
}
