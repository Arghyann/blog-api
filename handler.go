package main

import (
	"github.com/minio/minio-go/v7"
	"database/sql"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"os"
	"time"
	"modernc.org/sqlite"
	"errors"
)

type API struct {
	db *sql.DB
	r2 *minio.Client
}

func (a *API) listPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT ID, Title, Slug, Description, COALESCE(tags, ''), COALESCE(published_at, '') FROM posts ORDER BY published_at DESC`)
	if err != nil {
		log.Println("listPosts query failed: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	posts := []Post{}
	for rows.Next() {
		var p Post
		err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Description, &p.Tags, &p.PublishedAt)
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
	var tags string
	var publishedAt string
	var description string
	err := a.db.QueryRow(
		`SELECT ID, Title, Body, COALESCE(tags, ''), COALESCE(published_at, ''), COALESCE(description, '')
		 FROM posts 
		 WHERE Slug = ? `, slug).Scan(&id, &title, &body, &tags, &publishedAt, &description)
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
		"id":           id,
		"slug":         slug,
		"description":  description,
		"title":        title,
		"body":         body,
		"tags":         tags,
		"published_at": publishedAt,
	}
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(response)

}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		User     string `json:"user"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Days     int    `json:"days"` // optional: token lifetime in days (default 1 hour, max maxLoginDays)
	}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		log.Println("Login Error:", err)
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	username := input.Username
	if username == "" {
		username = input.User
	}
	if username == "" {
		username = input.Email
	}

	var passwordHash string
	var id string

	err = a.db.QueryRow(
		`SELECT password_hash, id FROM admins WHERE user = ?`,
		username,
	).Scan(&passwordHash, &id)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(input.Password),
	)

	if err != nil {
		log.Println("Invalid password")
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(tokenTTL(input.Days)).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	jwtSecret := os.Getenv("JWT_SECRET")

	signedToken, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		http.Error(w, "Could not create token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"token": signedToken,
	})
}

func (a *API) UploadPost(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title       string `json:"title"`
		Slug        string `json:"slug"`
		Body        string `json:"body"`
		Tags        string `json:"tags"`
		Description string `json:"description"`
	}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		log.Println("Invalid body", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if input.Title == "" || input.Slug == "" || input.Body == "" {
		http.Error(w, "title, slug, and body are required", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()

	_, err = a.db.Exec(
		`INSERT INTO posts
		(id, slug, title, body, tags, description)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id,
		input.Slug,
		input.Title,
		input.Body,
		input.Tags,
		input.Description,
	)

	if err != nil {
		var sqlError *sqlite.Error
		if errors.As(err, &sqlError) && sqlError.Code()==2067{
			log.Println("slug or title taken")
			http.Error(w,"Title or Slug already Take",http.StatusConflict)
			return
		}
		log.Println("Failed to create post:", err)
		http.Error(w, "Failed to create Post", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":   id,
		"slug": input.Slug,
		"tags": input.Tags,
	})
}

func (a *API) UpdatePost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var input struct {
		Title       string `json:"title"`
		Slug        string `json:"slug"`
		Body        string `json:"body"`
		Tags        string `json:"tags"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if input.Title == "" || input.Body == "" {
		http.Error(w, "title and body are required", http.StatusBadRequest)
		return
	}
	// slug in body is optional; lets you rename a post
	newSlug := input.Slug
	if newSlug == "" {
		newSlug = slug
	}

	res, err := a.db.Exec(
		`UPDATE posts
		 SET slug = ?, title = ?, body = ?, tags = ?, description = ?
		 WHERE slug = ?`,
		newSlug, input.Title, input.Body, input.Tags, input.Description, slug,
	)
	if err != nil {
		var sqlError *sqlite.Error
		if errors.As(err, &sqlError) && sqlError.Code() == 2067 {
			http.Error(w, "Title or Slug already taken", http.StatusConflict)
			return
		}
		log.Println("Failed to update post:", err)
		http.Error(w, "Failed to update Post", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "Post Not Found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"slug": newSlug})
}

func (a *API) DeletePost(w http.ResponseWriter, r *http.Request) {
	res, err := a.db.Exec(`DELETE FROM posts WHERE slug = ?`, r.PathValue("slug"))
	if err != nil {
		log.Println("Failed to delete post:", err)
		http.Error(w, "Failed to delete Post", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "Post Not Found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

const maxLoginDays = 30

// tokenTTL is the lifetime of a login token: 1 hour unless the client asks for
// a number of days, capped at maxLoginDays. The caller has already proven the password.
func tokenTTL(days int) time.Duration {
	if days <= 0 {
		return time.Hour
	}
	if days > maxLoginDays {
		days = maxLoginDays
	}
	return time.Duration(days) * 24 * time.Hour
}
