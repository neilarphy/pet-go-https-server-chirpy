package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type apiConfig struct {
	fileserverHits atomic.Int32
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

func cleanBadWords(text string) string{
	bad_words := map[string]string{
		"kerfuffle":"****",
		"sharbert":"****",
		"fornax":"****",
	}
	new_text := []string{}
	for _, word := range strings.Split(text, " "){
		if _, exists := bad_words[strings.ToLower(word)]; exists {
			new_text = append(new_text, bad_words[strings.ToLower(word)])
		}else{
			new_text = append(new_text, word)
		}
	}
	return strings.Join(new_text, " ")
}

func main() {

	config := apiConfig{}

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
		config.fileserverHits.Store(0)
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
			// respBody := errChirp{
			// 	Error: "Something went wrong",
			// }
			// dat, err := json.Marshal(respBody)
			// if err != nil {
			// 	log.Printf("Error marshaling JSON: %s", err)
			// 	w.WriteHeader(500)
			// 	return
			// }
			// w.Header().Set("Content-Type", "application/json")
			// w.WriteHeader(500)
			// if _, err := w.Write(dat); err != nil {
			// 	log.Printf("failed to write response: %v", err)
			// }
			// return
		}

		if params.Body == "" {
			respondWithError(w, 400, "Chirp cannot be empty")
			return
			// respBody := errChirp{
			// 	Error: "Chirp cannot be empty",
			// }
			// dat, err := json.Marshal(respBody)
			// if err != nil {
			// 	log.Printf("Error marshaling JSON: %s", err)
			// 	w.WriteHeader(500)
			// 	return
			// }
			// w.Header().Set("Content-Type", "application/json")
			// w.WriteHeader(400)
			// if _, err := w.Write(dat); err != nil {
			// 	log.Printf("failed to write response: %v", err)
			// }
			// return
		}

		if len(params.Body) > 140 {
			respondWithError(w, 400, "Chirp is too long")
			return
			// respBody := errChirp{
			// 	Error: "Chirp is too long",
			// }
			// dat, err := json.Marshal(respBody)
			// if err != nil {
			// 	log.Printf("Error marshaling JSON: %s", err)
			// 	w.WriteHeader(500)
			// 	return
			// }
			// w.Header().Set("Content-Type", "application/json")
			// w.WriteHeader(400)
			// if _, err := w.Write(dat); err != nil {
			// 	log.Printf("failed to write response: %v", err)
			// }
			// return
		}

		respBody := validChirp{
			Cleaned_body: cleanBadWords(params.Body),
		}
		respondWithJSON(w, 200, respBody)
		return
		// dat, err := json.Marshal(respBody)
		// if err != nil {
		// 	log.Printf("Error marshaling JSON: %s", err)
		// 	w.WriteHeader(400)
		// 	return
		// }
		// w.Header().Set("Content-Type", "application/json")
		// w.WriteHeader(200)
		// if _, err := w.Write(dat); err != nil {
		// 	log.Printf("failed to write response: %v", err)
		// }
		// return
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
