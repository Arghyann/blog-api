package main

import (
	"database/sql"
	"log"
	_ "modernc.org/sqlite"
)

func initDb(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`PRAGMA journal_mode=WAL;`)
	if err != nil {
		log.Fatal(err)
	}
	schema := `
	CREATE TABLE IF NOT EXISTS posts (
		id TEXT PRIMARY KEY,
		slug TEXT UNIQUE NOT NULL,
	title TEXT UNIQUE NOT NULL,
		body TEXT NOT NULL,
		tags TEXT,
		published_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		description TEXT NOT NULL
	);
	
	CREATE TABLE IF NOT EXISTS admins (
		id TEXT PRIMARY KEY,
		user TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL
	);`
	_, err = db.Exec(schema)
	if err != nil {
		log.Fatal(err)
	}

	return db
}
