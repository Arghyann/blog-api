package main

import "time"

type Post struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Body        string    `json:"body"`
	Tags        string    `json:"tags"`
	PublishedAt time.Time `json:"published_at"`
}
type Admin struct {
	ID           string `json:"id"`
	User         string `json:"user"`
	PasswordHash string `json:"PasswordHash"`
}
