package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// load .env if present (non-fatal if absent in production)
	_ = godotenv.Load()
	if os.Getenv("JWT_SECRET") == "" {
		log.Println("Warning: JWT_SECRET environment variable is not set")
	}

	db := initDb("./posts.db")
	defer db.Close()

	if len(os.Args) > 1 && os.Args[1] == "create-user" {
		createUser(db)
		return
	}

	api := &API{
		db: db,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /posts/{slug}", api.getPost)
	mux.HandleFunc("GET /posts", api.listPosts)
	mux.Handle("POST /posts", authorize(http.HandlerFunc(api.UploadPost)))
	mux.HandleFunc("POST /login", api.login)

	log.Println("Server running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			http.Error(w, "Unauthorized: missing token", http.StatusUnauthorized)
			return
		}

		// Strip "Bearer " prefix if provided
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")

		token, err := jwt.Parse(tokenString, func(_ *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !token.Valid {
			log.Println("Bad Token:", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
func createUser(db *sql.DB) {
	fmt.Print("Username: ")
	var username string
	fmt.Scanln(&username)
	fmt.Print("Password: ")
	var password string
	fmt.Scanln(&password)
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		log.Fatal(err)
	}
	id := uuid.New().String()
	_, err = db.Exec(
		`INSERT INTO admins (id, user, password_hash)
		 VALUES (?, ?, ?)`,
		id,
		username,
		hash,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("User created")
}
