package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/neilarphy/pet-go-https-server-chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	DB             *database.Queries
	PLATFORM       string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type error struct {
		Error string `json:"error"`
	}

	respBody := error{
		Error: msg,
	}
	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshaling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write(dat); err != nil {
		log.Printf("failed to write response: %v", err)
	}
	return

}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON: %s", err)
		w.WriteHeader(400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write(dat); err != nil {
		log.Printf("failed to write response: %v", err)
	}
	return
}

func cleanBadWords(text string) string {
	bad_words := map[string]string{
		"kerfuffle": "****",
		"sharbert":  "****",
		"fornax":    "****",
	}
	new_text := []string{}
	for _, word := range strings.Split(text, " ") {
		if _, exists := bad_words[strings.ToLower(word)]; exists {
			new_text = append(new_text, bad_words[strings.ToLower(word)])
		} else {
			new_text = append(new_text, word)
		}
	}
	return strings.Join(new_text, " ")
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	env := os.Getenv("PLATFORM")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {

	}

	dbQueries := database.New(db)

	config := apiConfig{
		DB:       dbQueries,
		PLATFORM: env,
	}

	mux := http.NewServeMux()
	mux.Handle("/app/", config.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	mux.Handle("/api/assets", http.FileServer(http.Dir(".")))
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("failed to write response: %v", err)
		}
	})
	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html>
						<body>
							<h1>Welcome, Chirpy Admin</h1>
							<p>Chirpy has been visited %d times!</p>
						</body>
						</html>`, config.fileserverHits.Load())
	})

	mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		if config.PLATFORM != "dev" {
			respondWithError(w, 403, "Forbidden")
			return
		}
		config.fileserverHits.Store(0)
		err := config.DB.DeleteAllUsers(r.Context())
		if err != nil {
			log.Fatal(err)
		}
	})

	mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		type checkChirp struct {
			Body string `json:"body"`
		}
		type errChirp struct {
			Error string `json:"error"`
		}
		type validChirp struct {
			Cleaned_body string `json:"cleaned_body"`
		}

		decoder := json.NewDecoder(r.Body)
		params := checkChirp{}
		if err := decoder.Decode(&params); err != nil {
			respondWithError(w, 500, "Something went wrong")
			return
		}

		if params.Body == "" {
			respondWithError(w, 400, "Chirp cannot be empty")
			return
		}

		if len(params.Body) > 140 {
			respondWithError(w, 400, "Chirp is too long")
			return
		}

		respBody := validChirp{
			Cleaned_body: cleanBadWords(params.Body),
		}
		respondWithJSON(w, 200, respBody)
		return
	})

	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		type users struct {
			Email string `json:"email"`
		}

		type userCreated struct {
			ID        uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Email     string    `json:"email"`
		}

		decoder := json.NewDecoder(r.Body)
		params := users{}
		if err := decoder.Decode(&params); err != nil {
			respondWithError(w, 500, "Something went wrong")
			return
		}

		if params.Email == "" {
			respondWithError(w, 400, "Email cannot be empty")
			return
		}

		user, err := config.DB.CreateUser(r.Context(), params.Email)
		if err != nil {
			respondWithError(w, 500, fmt.Sprintf("User was not created with error %v", err))
			return
		}

		respBody := userCreated{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email:     user.Email,
		}

		respondWithJSON(w, 201, respBody)
		return
	})

	server := &http.Server{
		Addr:           ":8080",
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(server.ListenAndServe())
}
