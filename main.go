package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var (
	db          *sql.DB
	shortLength = 6
	charset     = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	initDB()

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/shorten", shortenHandler)
	http.HandleFunc("/shorturl/", redirectHandler)
	http.HandleFunc("/api/visits", visitCountHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("🚀 Server đang chạy tại http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

// Kết nối đến database PostgreSQL
func initDB() {
	var err error
	connStr := "postgres://postgres:091123@localhost:5432/shortener?sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}
	if err := db.Ping(); err != nil {
		panic(err)
	}
	fmt.Println("✅ Đã kết nối thành công đến PostgreSQL")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("static/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cookie, err := r.Cookie("shortURL")
	shortURL := ""
	shortCode := ""
	visits := 0

	if err == nil {
		shortURL = cookie.Value
		shortCode = strings.TrimPrefix(shortURL, "http://localhost:8080/shorturl/")
		_ = db.QueryRow("SELECT visit_count FROM urls WHERE short_code = $1", shortCode).Scan(&visits)
	}

	data := struct {
		ShortURL  string
		Visits    int
		ShortCode string
		Shortened bool
	}{
		ShortURL:  shortURL,
		Visits:    visits,
		ShortCode: shortCode,
		Shortened: shortURL != "",
	}

	tmpl.Execute(w, data)
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	originalURL := r.URL.Query().Get("url")
	if originalURL == "" {
		http.Error(w, "Vui lòng nhập URL hợp lệ.", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(originalURL, "http://") && !strings.HasPrefix(originalURL, "https://") {
		originalURL = "https://www." + originalURL
	}

	shortCode := generateShortURL()
	shortURL := "http://localhost:8080/shorturl/" + shortCode

	_, err := db.Exec("INSERT INTO urls (short_code, original_url, visit_count) VALUES ($1, $2, 0)", shortCode, originalURL)
	if err != nil {
		http.Error(w, "Không thể lưu URL.", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "shortURL",
		Value: shortURL,
		Path:  "/",
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := strings.TrimPrefix(r.URL.Path, "/shorturl/")

	var originalURL string
	err := db.QueryRow("SELECT original_url FROM urls WHERE short_code = $1", shortCode).Scan(&originalURL)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_, _ = db.Exec("UPDATE urls SET visit_count = visit_count + 1 WHERE short_code = $1", shortCode)

	http.Redirect(w, r, originalURL, http.StatusFound)
}

func visitCountHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := r.URL.Query().Get("code")
	if shortCode == "" {
		http.Error(w, "Thiếu short code", http.StatusBadRequest)
		return
	}

	var visits int
	err := db.QueryRow("SELECT visit_count FROM urls WHERE short_code = $1", shortCode).Scan(&visits)
	if err != nil {
		http.Error(w, "Không tìm thấy mã rút gọn", http.StatusNotFound)
		return
	}

	resp := struct {
		Visits int `json:"visits"`
	}{Visits: visits}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func generateShortURL() string {
	b := make([]byte, shortLength)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
