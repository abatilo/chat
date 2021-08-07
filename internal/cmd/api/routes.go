package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) registerRoutes() {
	s.router.Get("/check", s.ping())
	s.router.Post("/users", s.createUser())
	s.router.Post("/login", s.login())
}

func (s *Server) ping() http.HandlerFunc {
	duration := s.metrics.NewHistogram(prometheus.HistogramOpts{
		Name: "chat_check_duration_seconds",
		Help: "Histogram for check endpoint latency",
	})

	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		fmt.Fprintf(w, "ok")
		duration.Observe(time.Since(startTime).Seconds())
	}
}

func (s *Server) createUser() http.HandlerFunc {
	log := s.logger.With().Str("method", "createUser").Logger()

	duration := s.metrics.NewHistogram(prometheus.HistogramOpts{
		Name: "chat_create_users_duration_seconds",
		Help: "Histogram for createUser endpoint latency",
	})

	type createUserRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	type createUserResponse struct {
		ID int64 `json:"id"`
	}

	const (
		insertQueryString = "INSERT INTO chat_user (username, password) VALUES ($1, $2) returning chat_user_id"
	)

	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		var req createUserRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			err := fmt.Errorf("Couldn't decode req: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		io.CopyN(ioutil.Discard, r.Body, 512)
		r.Body.Close()

		// TODO: Request param length validation

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.MinCost)
		if err != nil {
			err := fmt.Errorf("Couldn't hash the passed in password: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tx, err := s.db.Begin(r.Context())
		if err != nil {
			err := fmt.Errorf("Couldn't begin transaction: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var userID int64
		err = tx.QueryRow(r.Context(), insertQueryString, req.Username, hashedPassword).Scan(&userID)
		if err != nil {
			err := fmt.Errorf("Couldn't insert new user: %w", err)
			log.Err(err).Msg(err.Error())
			tx.Rollback(r.Context())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tx.Commit(r.Context())
		resp := createUserResponse{ID: int64(userID)}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)

		duration.Observe(time.Since(startTime).Seconds())
	}
}

func (s *Server) login() http.HandlerFunc {
	log := s.logger.With().Str("method", "login").Logger()

	duration := s.metrics.NewHistogram(prometheus.HistogramOpts{
		Name: "chat_login_duration_seconds",
		Help: "Histogram for login endpoint latency",
	})

	type loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	type loginResponse struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}

	const (
		selectPasswordQueryString = "SELECT chat_user_id, password from chat_user WHERE username = $1"
	)

	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		var req loginRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			err := fmt.Errorf("Couldn't decode req: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		io.CopyN(ioutil.Discard, r.Body, 512)
		r.Body.Close()

		// TODO: Request param length validation

		if err != nil {
			err := fmt.Errorf("Couldn't begin transaction: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var userID int64
		var hashedPassword string
		err = s.db.QueryRow(r.Context(), selectPasswordQueryString, req.Username).Scan(&userID, &hashedPassword)
		if err != nil {
			err := fmt.Errorf("Couldn't fetch user's hashed password: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
		if err != nil {
			err := fmt.Errorf("Incorrect password: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		token := uuid.New()

		s.sessionManager.Put(r.Context(), "token", token.String())

		resp := loginResponse{ID: int64(userID), Token: token.String()}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)

		duration.Observe(time.Since(startTime).Seconds())
	}
}
