// blogctl: a small CLI for writing and editing posts on the blog API.
//
//	blogctl login
//	blogctl list
//	blogctl new                 open $EDITOR with a template, then publish
//	blogctl push <file|name>    publish (or update, if the slug exists) a note, by path or by
//	                            exact filename/title/slug found in your vault
//	blogctl edit <slug>         fetch, open in $EDITOR, update on save
//	blogctl delete <slug>
//	blogctl image [file]        upload an image (asks for a path if omitted), print its markdown snippet
//
// Post files are markdown with a front-matter header:
//
//	---
//	title: My post
//	slug: my-post
//	description: One line summary
//	tags: Go, APIs
//	---
//	body...
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultURL = "https://aryanssh.duckdns.org/blog-api"

type config struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	Vault string `json:"vault,omitempty"`
}

type post struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at,omitempty"`
}

const template = `---
title: 
slug: 
description: 
tags: 
---

`

func configPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "blogctl", "config.json")
}

func loadConfig() config {
	c := config{URL: defaultURL}
	if b, err := os.ReadFile(configPath()); err == nil {
		json.Unmarshal(b, &c)
	}
	if v := os.Getenv("BLOG_API_URL"); v != "" {
		c.URL = v
	}
	c.URL = strings.TrimRight(c.URL, "/")
	return c
}

func saveConfig(c config) {
	p := configPath()
	os.MkdirAll(filepath.Dir(p), 0o700)
	b, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		die("saving config: %v", err)
	}
}

func die(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+f+"\n", a...)
	os.Exit(1)
}

// do sends a request and returns the body; non-2xx becomes an error.
func (c config) do(method, path, contentType string, body io.Reader) []byte {
	req, err := http.NewRequest(method, c.URL+path, body)
	if err != nil {
		die("%v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		die("%v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		die("unauthorized: run `blogctl login` (token expires after 1 hour)")
	}
	if resp.StatusCode >= 300 {
		die("%s %s: %s %s", method, path, resp.Status, strings.TrimSpace(string(data)))
	}
	return data
}

func (c config) jsonReq(method, path string, v any) []byte {
	b, _ := json.Marshal(v)
	return c.do(method, path, "application/json", bytes.NewReader(b))
}

func prompt(label string) string {
	fmt.Print(label)
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(s)
}

func promptSecret(label string) string {
	fmt.Print(label)
	stty := func(arg string) {
		cmd := exec.Command("stty", arg)
		cmd.Stdin = os.Stdin
		cmd.Run()
	}
	stty("-echo")
	defer func() { stty("echo"); fmt.Println() }()
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(s)
}

func parsePost(text string) (post, error) {
	var p post
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return p, fmt.Errorf("missing front matter (file must start with ---)")
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return p, fmt.Errorf("front matter is not closed with ---")
	}
	var tags []string
	inTags := false
	for _, line := range strings.Split(rest[:end], "\n") {
		if t := strings.TrimSpace(line); inTags && strings.HasPrefix(t, "- ") {
			tags = append(tags, unquote(strings.TrimPrefix(t, "- ")))
			continue
		}
		inTags = false
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = unquote(strings.TrimSpace(v))
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "title":
			p.Title = v
		case "slug":
			p.Slug = v
		case "description":
			p.Description = v
		case "tags":
			if v == "" {
				inTags = true
			} else {
				p.Tags = strings.Trim(v, "[]")
			}
		}
	}
	if len(tags) > 0 {
		p.Tags = strings.Join(tags, ", ")
	}
	p.Body = strings.TrimSpace(rest[end+4:])
	if p.Title == "" || p.Slug == "" || p.Body == "" {
		return p, fmt.Errorf("title, slug and body are required")
	}
	return p, nil
}

func formatPost(p post) string {
	return fmt.Sprintf("---\ntitle: %s\nslug: %s\ndescription: %s\ntags: %s\n---\n\n%s\n",
		p.Title, p.Slug, p.Description, p.Tags, p.Body)
}

// editText opens text in $EDITOR and returns the result.
func editText(initial string) string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}
	f, err := os.CreateTemp("", "blogctl-*.md")
	if err != nil {
		die("%v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(initial)
	f.Close()
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], f.Name())...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		die("editor: %v", err)
	}
	b, _ := os.ReadFile(f.Name())
	return string(b)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: blogctl <login|list|new|push|edit|delete|image> [args]")
		os.Exit(2)
	}
	c := loadConfig()
	args := os.Args[2:]

	switch os.Args[1] {
	case "login":
		user := prompt("Username: ")
		pass := promptSecret("Password: ")
		out := c.jsonReq("POST", "/login", map[string]string{"username": user, "password": pass})
		var r struct{ Token string }
		json.Unmarshal(out, &r)
		if r.Token == "" {
			die("no token in response")
		}
		c.Token = r.Token
		saveConfig(c)
		fmt.Println("logged in (token valid for 1 hour)")

	case "list":
		var posts []post
		json.Unmarshal(c.do("GET", "/posts", "", nil), &posts)
		for _, p := range posts {
			fmt.Printf("%-40s %s\n", p.Slug, p.Title)
		}

	case "new":
		publish(c, editText(template), false)

	case "push":
		if len(args) < 1 {
			die("usage: blogctl push <file.md>")
		}
		path := findNote(&c, args[0])
		b, err := os.ReadFile(path)
		if err != nil {
			die("%v", err)
		}
		fmt.Println("pushing", path)
		publish(c, string(b), true)

	case "edit":
		if len(args) < 1 {
			die("usage: blogctl edit <slug>")
		}
		var p post
		json.Unmarshal(c.do("GET", "/posts/"+args[0], "", nil), &p)
		if p.Slug == "" {
			p.Slug = args[0]
		}
		orig := formatPost(p)
		edited := editText(orig)
		if edited == orig {
			fmt.Println("no changes")
			return
		}
		np, err := parsePost(edited)
		if err != nil {
			dump(edited)
			die("%v", err)
		}
		c.jsonReq("PUT", "/posts/"+args[0], np)
		fmt.Println("updated", np.Slug)

	case "delete":
		if len(args) < 1 {
			die("usage: blogctl delete <slug>")
		}
		if prompt(fmt.Sprintf("Delete %q? type the slug to confirm: ", args[0])) != args[0] {
			die("aborted")
		}
		c.do("DELETE", "/posts/"+args[0], "", nil)
		fmt.Println("deleted", args[0])

	case "image":
		path := ""
		if len(args) > 0 {
			path = args[0]
		} else {
			path = prompt("Image path: ")
		}
		if path = cleanPath(path); path == "" {
			die("no image path given")
		}
		uploadImage(c, path)

	default:
		die("unknown command %q", os.Args[1])
	}
}

// publish creates the post, or updates it when the slug already exists
// (only if allowUpdate, so `new` never silently overwrites).
func publish(c config, text string, allowUpdate bool) {
	p, err := parsePost(text)
	if err != nil {
		dump(text)
		die("%v", err)
	}
	if allowUpdate {
		req, _ := http.NewRequest("GET", c.URL+"/posts/"+p.Slug, nil)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				c.jsonReq("PUT", "/posts/"+p.Slug, p)
				fmt.Println("updated", p.Slug)
				return
			}
		}
	}
	c.jsonReq("POST", "/post", p)
	fmt.Println("published", p.Slug)
}

// dump saves rejected text so a failed edit is never lost.
func dump(text string) {
	f := filepath.Join(os.TempDir(), "blogctl-failed.md")
	os.WriteFile(f, []byte(text), 0o600)
	fmt.Fprintln(os.Stderr, "your text was saved to", f)
}

func uploadImage(c config, path string) {
	f, err := os.Open(path)
	if err != nil {
		die("%v", err)
	}
	defer f.Close()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename=%q`, filepath.Base(path)))
	part, _ := mw.CreatePart(h)
	io.Copy(part, f)
	mw.Close()
	out := c.do("POST", "/images", mw.FormDataContentType(), &buf)
	var r struct{ URL, Markdown string }
	json.Unmarshal(out, &r)
	fmt.Println(r.Markdown)
}

// unquote strips one pair of surrounding YAML quotes.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, `\"`, `"`)
	}
	return s
}

// cleanPath tidies a typed or drag-and-dropped path: surrounding quotes,
// backslash-escaped spaces and a leading ~ are all handled.
func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `"'`)
	p = strings.ReplaceAll(p, `\ `, " ")
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// findNote resolves a push argument to a file. An existing path is used as is;
// otherwise the vault is searched for an exact (case-insensitive) match on
// filename, front-matter title or slug. No fuzzy matching: push overwrites.
func findNote(c *config, arg string) string {
	if p := cleanPath(arg); fileExists(p) {
		return p
	}
	if c.Vault == "" {
		home, _ := os.UserHomeDir()
		def := filepath.Join(home, "Documents", "Obsidian Vault")
		v := prompt(fmt.Sprintf("Vault folder [%s]: ", def))
		if v == "" {
			v = def
		}
		c.Vault = cleanPath(v)
		if st, err := os.Stat(c.Vault); err != nil || !st.IsDir() {
			die("%s is not a folder", c.Vault)
		}
		saveConfig(*c)
	}
	want := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(arg), ".md"))
	var matches []string
	filepath.WalkDir(c.Vault, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != c.Vault && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		name := strings.ToLower(strings.TrimSuffix(d.Name(), ".md"))
		var title, slug string
		if b, err := os.ReadFile(p); err == nil {
			if np, _ := parsePost(string(b)); np.Slug != "" {
				title, slug = strings.ToLower(np.Title), strings.ToLower(np.Slug)
			}
		}
		if name == want || (title != "" && title == want) || (slug != "" && slug == want) {
			matches = append(matches, p)
		}
		return nil
	})
	switch len(matches) {
	case 0:
		die("no note matching %q in %s", arg, c.Vault)
	case 1:
		return matches[0]
	}
	fmt.Fprintf(os.Stderr, "%q matches several notes:\n", arg)
	for _, m := range matches {
		fmt.Fprintln(os.Stderr, "  ", m)
	}
	os.Exit(1)
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
