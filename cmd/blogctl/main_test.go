package main

import (
	"os"
	"path/filepath"
	"strings"
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
		"Tailscale and the networking Rabbithole": "Tailscale and the networking Rabbithole.md",
		"tailscale and the networking rabbithole": "Tailscale and the networking Rabbithole.md",
		"Fine tuning":                           "Fine tuning.md",
		"blockchain":                            "Blockchain and why.md",
		"microservice-million-rps":              "Why?.md",
		"Why I Built a Blockchain from Scratch": "Blockchain and why.md",
	} {
		if got := filepath.Base(findNote(c, arg)); got != want {
			t.Errorf("findNote(%q) = %q, want %q", arg, got, want)
		}
	}
}

func TestEnvRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("BLOG_TOKEN", "")
	t.Setenv("BLOG_API_URL", "")
	saveConfig(config{fileURL: defaultURL, fileToken: "tok.en.x", Vault: "/v/my vault"})
	got := loadConfig()
	if got.Token != "tok.en.x" || got.Vault != "/v/my vault" || got.URL != defaultURL {
		t.Fatalf("round trip lost data: %+v", got)
	}
	if st, _ := os.Stat(envPath()); st.Mode().Perm() != 0o600 {
		t.Errorf("config mode %v, want 600", st.Mode().Perm())
	}
	t.Setenv("BLOG_TOKEN", "override")
	t.Setenv("BLOG_API_URL", "http://localhost:1/x")
	o := loadConfig()
	if o.Token != "override" || o.URL != "http://localhost:1/x" {
		t.Error("environment should override the file")
	}
	saveConfig(o) // must not write the overrides back
	b, _ := os.ReadFile(envPath())
	if strings.Contains(string(b), "override") || strings.Contains(string(b), "localhost") {
		t.Errorf("environment override leaked into the file:\n%s", b)
	}
}

func TestTokenExpiry(t *testing.T) {
	tok := "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjQxMDI0NDQ4MDB9.sig" // exp 2100-01-01
	if exp, err := tokenExpiry(tok); err != nil || exp.Year() != 2100 {
		t.Fatalf("got %v, %v", exp, err)
	}
	for _, bad := range []string{"", "abc", "a.b.c", "a.e30.c"} {
		if _, err := tokenExpiry(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
