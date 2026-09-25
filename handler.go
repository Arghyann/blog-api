package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

type API struct {
	db *sql.DB
}

func (a *API) listPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT ID,Title,Slug,Description FROM posts`)
	if err != nil {
		log.Println("listPosts query failed: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	posts := []Post{}
	for rows.Next() {
		var p Post
		err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Description)
		if err != nil {
			log.Println("Error while listingPosts", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		log.Println("Error reading rows: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func (a *API) getPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id string
	var title string
	var body string
	err := a.db.QueryRow(
		`SELECT ID, Title, Body
		 FROM posts 
		 WHERE Slug = ? `, slug).Scan(&id, &title, &body)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Error while reading post", err)

			http.Error(w, "Post Not Found", http.StatusNotFound)
			return

		}
		http.Error(w, "Database Error", http.StatusInternalServerError)
		return

	}
	response := map[string]interface{}{
		"id":    id,
		"Title": title,
		"Body":  body,
	}
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(response)

}
