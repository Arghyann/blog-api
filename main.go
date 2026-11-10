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
	"os/exec"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
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
	if len(os.Args) > 1 && os.Args[1] == "change-password" {
		changePassword(db)
		return
	}
	api := &API{
		db: db,
		r2: initR2(),
	}
	mux := http.NewServeMux()
	mux.Handle("GET /blog-api/posts/{slug}", MetricsMiddleware("/blog-api/posts/{slug}", http.HandlerFunc(api.getPost)))
	mux.Handle("GET /blog-api/posts", MetricsMiddleware("/blog-api/posts", http.HandlerFunc(api.listPosts)))
	mux.Handle("POST /blog-api/post", MetricsMiddleware("/blog-api/post", authorize(http.HandlerFunc(api.UploadPost))))
	mux.Handle("PUT /blog-api/posts/{slug}", MetricsMiddleware("/blog-api/posts/{slug}", authorize(http.HandlerFunc(api.UpdatePost))))
	mux.Handle("DELETE /blog-api/posts/{slug}", MetricsMiddleware("/blog-api/posts/{slug}", authorize(http.HandlerFunc(api.DeletePost))))
	mux.Handle("POST /blog-api/images", MetricsMiddleware("/blog-api/images", authorize(http.HandlerFunc(api.UploadImage))))
	mux.Handle("POST /blog-api/login", MetricsMiddleware("/blog-api/login", http.HandlerFunc(api.login)))
	mux.Handle("GET /blog-api/metrics", promhttp.Handler())
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

// readSecret prompts for input without echoing it to the terminal.
func readSecret(label string) string {
	fmt.Print(label)
	stty := func(arg string) {
		cmd := exec.Command("stty", arg)
		cmd.Stdin = os.Stdin
		cmd.Run()
	}
	stty("-echo")
	defer func() { stty("echo"); fmt.Println() }()
	return readLine()
}

// readLine reads one line from stdin a byte at a time, so it never swallows
// input meant for a later prompt (or for fmt.Scanln).
func readLine() string {
	var sb strings.Builder
	b := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(b)
		if n == 1 {
			if b[0] == '\n' {
				break
			}
			sb.WriteByte(b[0])
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(sb.String())
}

// changePassword updates an existing admin's password: ./server change-password
func changePassword(db *sql.DB) {
	fmt.Print("Username: ")
	var username string
	fmt.Scanln(&username)
	var exists bool
	if err := db.QueryRow(`SELECT 1 FROM admins WHERE user = ?`, username).Scan(&exists); err != nil {
		log.Fatal("No such user: ", username)
	}
	password := readSecret("New password: ")
	if len(password) < 12 {
		log.Fatal("Password must be at least 12 characters")
	}
	if readSecret("Confirm password: ") != password {
		log.Fatal("Passwords do not match")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE admins SET password_hash = ? WHERE user = ?`, hash, username); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Password updated for", username)
}
