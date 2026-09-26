package main

import (
	"log"
	"net/http"
	"os"
	"fmt"
	"database/sql"
	"golang.org/x/crypto/bcrypt"
	"github.com/joho/godotenv"
	"github.com/google/uuid"
	"github.com/golang-jwt/jwt/v5"
	"strings"
)

func main() {
	//connect to db
	err := godotenv.Load(".env")
	if err!=nil{
		log.Fatal("Couldn't load .env")
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
	mux.HandleFunc("GET /blog-api/posts/{slug}", api.getPost)
	mux.HandleFunc("GET /blog-api/posts", api.listPosts)
	mux.Handle("POST /blog-api/post",authorize(http.HandlerFunc(api.UploadPost)))
	mux.HandleFunc("POST /blog-api/login",api.login)
	// 4. Start the HTTP server
	log.Println("Server running on port 8080")

	log.Fatal(http.ListenAndServe(":8081", mux))
}

func authorize(next http.Handler) http.Handler{
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		tokenString := r.Header.Get("Authorization")
		tokenString,_=strings.CutPrefix(tokenString,"Bearer ")
		token,err:=jwt.Parse(tokenString,func(_ *jwt.Token)(interface{},error){
			return []byte(os.Getenv("JWT_SECRET")),nil
		})
		if err!=nil || !token.Valid{
		log.Println("Bad Token")
		http.Error(w,"Unauthorized",http.StatusUnauthorized)
		return
		}
		next.ServeHTTP(w,r)

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
