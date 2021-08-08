package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) registerRoutes() {
	s.router.Use(s.sessionManager.LoadAndSave)
	s.router.Get("/check", s.ping())
	s.router.Post("/users", s.createUser())
	s.router.Post("/login", s.login())
	s.router.Route("/messages", func(r chi.Router) {
		r.Use(s.authRequired())
		r.Post("/", s.createMessage())
		r.Get("/", s.listMessages())
	})
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
		insertQueryString = "INSERT INTO chat_user (username, password) VALUES ($1, $2) returning id"
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
		selectPasswordQueryString = "SELECT id, password from chat_user WHERE username = $1"
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
		var hashedPassword []byte
		err = s.db.QueryRow(r.Context(), selectPasswordQueryString, req.Username).Scan(&userID, &hashedPassword)
		if err != nil {
			err := fmt.Errorf("Couldn't fetch user's hashed password: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(req.Password))
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
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)

		duration.Observe(time.Since(startTime).Seconds())
	}
}

func (s *Server) authRequired() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorizationHeader := r.Header.Get("authorization")
			if authorizationHeader == "" {
				// We might want these as 404s. It depends on the expected user experience
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			if strings.HasPrefix(strings.ToLower(authorizationHeader), "bearer ") {
				authorizationHeader = authorizationHeader[len("bearer "):]
			}

			token := s.sessionManager.GetString(r.Context(), "token")

			if token == authorizationHeader {
				next.ServeHTTP(w, r)
			} else {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			}
		})
	}
}

func (s *Server) createMessage() http.HandlerFunc {
	log := s.logger.With().Str("method", "createMessage").Logger()

	duration := s.metrics.NewHistogram(prometheus.HistogramOpts{
		Name: "chat_create_message_duration_seconds",
		Help: "Histogram for createMessage endpoint latency",
	})

	type content struct {
		Type string `json:"type"`
		// Type == "text"
		Text string `json:"text,omitempty"`

		// Type == "image"
		Height uint64 `json:"height,omitempty"`
		Width  uint64 `json:"width,omitempty"`

		// Type == "video"
		Source string `json:"source,omitempty"`

		// Type == "image" || Type == "video"
		URL string `json:"url,omitempty"`
	}

	type createMessageRequest struct {
		Sender    int64   `json:"sender"`
		Recipient int64   `json:"recipient"`
		Content   content `json:"content"`
	}

	type createMessageReponse struct {
		ID        int64  `json:"id"`
		Timestamp string `json:"timestamp"`
	}

	const (
		listMessageTypesQueryString   = "SELECT id, name FROM message_type"
		listVideoSourcesQueryString   = "SELECT id, name FROM video_source"
		createMessageQueryString      = "INSERT INTO message (sender_id, recipient_id, message_type_id) VALUES ($1, $2, $3) RETURNING id, created_at"
		createTextMessageQueryString  = "INSERT INTO text_message (message_id, text) VALUES ($1, $2)"
		createImageMessageQueryString = "INSERT INTO image_message (message_id, url, width, height) VALUES ($1, $2, $3, $4)"
		createVideoMessageQueryString = "INSERT INTO video_message (message_id, url, source) VALUES ($1, $2, $3)"
	)

	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		var req createMessageRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			err := fmt.Errorf("Couldn't decode req: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		io.CopyN(ioutil.Discard, r.Body, 512)
		r.Body.Close()

		tx, err := s.db.Begin(r.Context())
		if err != nil {
			err := fmt.Errorf("Couldn't begin transaction: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		messageNameToID := make(map[string]int16)

		rows, err := tx.Query(r.Context(), listMessageTypesQueryString)
		if err != nil {
			err := fmt.Errorf("Couldn't lookup valid message types: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			tx.Rollback(r.Context())
			return
		}
		for rows.Next() {
			var messageTypeID int16
			var messageTypeName string
			rows.Scan(&messageTypeID, &messageTypeName)
			messageNameToID[messageTypeName] = messageTypeID
		}

		messageTypeID, found := messageNameToID[req.Content.Type]
		if !found {
			err := fmt.Errorf("Unrecognized message type: %v", req.Content.Type)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			tx.Rollback(r.Context())
			return
		}

		var messageID int64
		var createdAt time.Time
		err = tx.QueryRow(r.Context(), createMessageQueryString, req.Sender, req.Recipient, messageTypeID).Scan(&messageID, &createdAt)
		if err != nil {
			err := fmt.Errorf("Couldn't create message: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			tx.Rollback(r.Context())
			return
		}

		if req.Content.Type == "text" {
			_, err = tx.Exec(r.Context(), createTextMessageQueryString, messageID, req.Content.Text)
			if err != nil {
				err := fmt.Errorf("Couldn't insert text message: %w", err)
				log.Err(err).Msg(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				tx.Rollback(r.Context())
				return
			}
		} else if req.Content.Type == "image" {
			_, err = tx.Exec(r.Context(), createImageMessageQueryString, messageID, req.Content.URL, req.Content.Width, req.Content.Height)
			if err != nil {
				err := fmt.Errorf("Couldn't insert image message: %w", err)
				log.Err(err).Msg(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				tx.Rollback(r.Context())
				return
			}
		} else if req.Content.Type == "video" {
			videoSourceToID := make(map[string]int16)

			rows, err := tx.Query(r.Context(), listVideoSourcesQueryString)
			if err != nil {
				err := fmt.Errorf("Couldn't lookup valid video sources: %w", err)
				log.Err(err).Msg(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				tx.Rollback(r.Context())
				return
			}
			for rows.Next() {
				var videoSourceID int16
				var videoSourceName string
				rows.Scan(&videoSourceID, &videoSourceName)
				videoSourceToID[videoSourceName] = videoSourceID
			}

			videoSourceID, found := videoSourceToID[req.Content.Source]
			if !found {
				err := fmt.Errorf("Unrecognized video source: %v", req.Content.Source)
				log.Err(err).Msg(err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				tx.Rollback(r.Context())
				return
			}

			_, err = tx.Exec(r.Context(), createVideoMessageQueryString, messageID, req.Content.URL, videoSourceID)
			if err != nil {
				err := fmt.Errorf("Couldn't insert video message: %w", err)
				log.Err(err).Msg(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				tx.Rollback(r.Context())
				return
			}
		} else {
			err := fmt.Errorf("Unrecognized message type: %v", req.Content.Type)
			log.Err(err).Msg(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			tx.Rollback(r.Context())
			return
		}

		tx.Commit(r.Context())

		resp := createMessageReponse{
			ID:        messageID,
			Timestamp: createdAt.String(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)

		duration.Observe(time.Since(startTime).Seconds())
	}
}

func (s *Server) listMessages() http.HandlerFunc {
	// log := s.logger.With().Str("method", "listMessages").Logger()

	duration := s.metrics.NewHistogram(prometheus.HistogramOpts{
		Name: "chat_list_messages_duration_seconds",
		Help: "Histogram for listMessages endpoint latency",
	})

	type loginRequest struct {
		Recipient int64 `json:"recipient"`
		Start     int64 `json:"start"`
		Limit     int64 `json:"limit"`
	}

	type loginResponse struct {
	}

	const (
		listMessagesQueryString = "SELECT chat_user_id, password from chat_user WHERE username = $1"
	)

	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		fmt.Fprintf(w, "listMessages")
		duration.Observe(time.Since(startTime).Seconds())
	}
}
