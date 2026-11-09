package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseVault(t *testing.T) {
	dir := os.Getenv("VAULT")
	if dir == "" {
		t.Skip("set VAULT")
	}
	for _, f := range []string{"Fine tuning.md", "Blockchain/Blockchain and why.md", "Tailscale and the networking Rabbithole.md", "microservice/Why?.md"} {
		b, _ := os.ReadFile(dir + "/" + f)
		p, err := parsePost(string(b))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		t.Logf("%s | %q | %q | tags=%q | body=%d", p.Slug, p.Title, p.Description[:20], p.Tags, len(p.Body))
	}
}

func TestCleanPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := map[string]string{
		`  "/a/b c.png" `: "/a/b c.png",
		`/a/b\ c.png`:     "/a/b c.png",
		`~/pics/x.png`:    home + "/pics/x.png",
		`'/a/x.png'`:      "/a/x.png",
	}
	for in, want := range cases {
		if got := cleanPath(in); got != want {
			t.Errorf("cleanPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindNote(t *testing.T) {
	dir := os.Getenv("VAULT")
	if dir == "" {
		t.Skip("set VAULT")
	}
	c := &config{Vault: dir}
	for arg, want := range map[string]string{
		"tailscale":                "Tailscale and the networking Rabbithole.md",
		"Fine tuning":              "Fine tuning.md",
		"blockchain":               "Blockchain and why.md",
		"microservice-million-rps": "Why?.md",
		"load testing a go micro":  "Why?.md",
	} {
		if got := filepath.Base(findNote(c, arg)); got != want {
			t.Errorf("findNote(%q) = %q, want %q", arg, got, want)
		}
	}
}
