package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"
)

// BEGIN registerRoutes

func (s *Server) registerRoutes() {
	// Register session middleware
	s.router.Use(s.sessionManager.LoadAndSave)

	// Application routes
	s.router.Get("/check", s.ping())
	s.router.Post("/users", s.createUser())
	s.router.Post("/login", s.login())
	s.router.Route("/messages", func(r chi.Router) {
		r.Use(s.authRequired())
		r.Post("/", s.createMessage())
		r.Get("/", s.listMessages())
	})
}

// END registerRoutes

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

		// Parse request
		var requestStruct createUserRequest
		bodyBytes, _ := ioutil.ReadAll(r.Body)
		r.Body.Close()
		json.Unmarshal(bodyBytes, &requestStruct)

		// Create user in database
		var userID int64
		tx, _ := s.db.Begin(r.Context())
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(requestStruct.Password), bcrypt.MinCost)
		tx.QueryRow(r.Context(), insertQueryString, requestStruct.Username, hashedPassword).Scan(&userID)
		tx.Commit(r.Context())

		// Write out response
		responseStruct := createUserResponse{ID: int64(userID)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(responseStruct)

		duration.Observe(time.Since(startTime).Seconds())
	}
}

func (s *Server) login() http.HandlerFunc {
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
		selectPasswordQueryString = "SELECT id, password FROM chat_user WHERE username = $1"
	)

	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Parse request
		var requestStruct loginRequest
		bodyBytes, _ := ioutil.ReadAll(r.Body)
		r.Body.Close()
		json.Unmarshal(bodyBytes, &requestStruct)

		var userID int64
		var hashedPassword []byte
		s.db.QueryRow(r.Context(), selectPasswordQueryString, requestStruct.Username).Scan(&userID, &hashedPassword)
		err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(requestStruct.Password))
		if err != nil {
			// Do we want to 401? 403?
			http.Error(w, "Failed to login", http.StatusUnauthorized)
			return
		}

		// Place a session API token into this user's session
		token := uuid.New()
		s.sessionManager.Put(r.Context(), "token", token.String())

		responseStruct := loginResponse{ID: int64(userID), Token: token.String()}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responseStruct)

		duration.Observe(time.Since(startTime).Seconds())
	}
}

func (s *Server) authRequired() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorizationHeader := r.Header.Get("authorization")
			if authorizationHeader == "" {
				// We might want these as 404s. It depends on the expected user experience
				s.logger.Error().Msg("Missing authorization header")
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
				s.logger.Error().Str("token", token).Str("header", authorizationHeader).Msg("Session token didn't match what's in authorization header")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			}
		})
	}
}

func (s *Server) createMessage() http.HandlerFunc {
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
		createMessageQueryString = `
INSERT INTO message (sender_id, recipient_id, message_type_id)
	SELECT $1, $2, message_type.id
		FROM message_type
		WHERE message_type.name = $3
	RETURNING id, created_at
`
		createTextMessageQueryString  = "INSERT INTO text_message (message_id, text) VALUES ($1, $2)"
		createImageMessageQueryString = "INSERT INTO image_message (message_id, url, width, height) VALUES ($1, $2, $3, $4)"
		createVideoMessageQueryString = `
INSERT INTO video_message (message_id, url, source)
	SELECT $1, $2, video_source.id
	FROM video_source
	WHERE video_source.name = $3
`
	)

	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Parse request
		var requestStruct createMessageRequest
		bodyBytes, _ := ioutil.ReadAll(r.Body)
		r.Body.Close()
		json.Unmarshal(bodyBytes, &requestStruct)

		if requestStruct.Content.Width == 0 {
			requestStruct.Content.Width = 64
		}
		if requestStruct.Content.Height == 0 {
			requestStruct.Content.Height = 64
		}

		tx, _ := s.db.Begin(r.Context())

		var messageID int64
		var createdAt time.Time
		tx.QueryRow(r.Context(),
			createMessageQueryString,
			requestStruct.Sender,
			requestStruct.Recipient,
			requestStruct.Content.Type).Scan(&messageID, &createdAt)

		if requestStruct.Content.Type == "text" {
			tx.Exec(r.Context(), createTextMessageQueryString, messageID, requestStruct.Content.Text)
		} else if requestStruct.Content.Type == "image" {
			tx.Exec(r.Context(), createImageMessageQueryString, messageID, requestStruct.Content.URL, requestStruct.Content.Width, requestStruct.Content.Height)
		} else if requestStruct.Content.Type == "video" {
			tx.Exec(r.Context(), createVideoMessageQueryString, messageID, requestStruct.Content.URL, requestStruct.Content.Source)
		}

		tx.Commit(r.Context())
		responseStruct := createMessageReponse{
			ID:        messageID,
			Timestamp: createdAt.UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(responseStruct)

		duration.Observe(time.Since(startTime).Seconds())
	}
}

func (s *Server) listMessages() http.HandlerFunc {
	duration := s.metrics.NewHistogram(prometheus.HistogramOpts{
		Name: "chat_list_messages_duration_seconds",
		Help: "Histogram for listMessages endpoint latency",
	})

	type listMessagesRequest struct {
		Recipient int64 `json:"recipient"`
		Start     int64 `json:"start"`
		Limit     int64 `json:"limit"`
	}

	type listMessagesResponseMessage struct {
		ID        int64                  `json:"id"`
		Sender    int64                  `json:"sender"`
		Recipient int64                  `json:"recipient"`
		Timestamp time.Time              `json:"timestamp"`
		Content   map[string]interface{} `json:"content"`
	}

	type listMessagesResponse struct {
		Messages []listMessagesResponseMessage `json:"messages"`
	}

	const (
		listMessagesQueryString = `
WITH desired_messages AS (
	SELECT id AS message_id
	FROM message
	WHERE recipient_id = $1
		AND id >= $2
	limit $3
)
SELECT message.id,
			 message.sender_id,
			 message.recipient_id,
			 message.created_at,
			 json_build_object(
				'type', message_type.name,
				'text', text_message.text
			 ) AS content
	FROM message
		join desired_messages ON message.id = desired_messages.message_id
		join message_type ON message.message_type_id = message_type.id
		join text_message ON message.id = text_message.message_id
UNION ALL
SELECT message.id,
			 message.sender_id,
			 message.recipient_id,
			 message.created_at,
			 json_build_object(
				'type',     message_type.name,
				'url',      image_message.url,
				'width',    image_message.width,
				'height',   image_message.height
			 ) AS content
	FROM message
		join desired_messages ON message.id = desired_messages.message_id
		join message_type ON message.message_type_id = message_type.id
		join image_message ON message.id = image_message.message_id
UNION ALL
SELECT message.id,
			 message.sender_id,
			 message.recipient_id,
			 message.created_at,
			 json_build_object(
				'type',     message_type.name,
				'url',      video_message.url,
				'source',   video_message.source
			 ) AS content
	FROM message
		join desired_messages ON message.id = desired_messages.message_id
		join message_type ON message.message_type_id = message_type.id
		join video_message ON message.id = video_message.message_id
		join video_source ON video_source.id = video_message.source
ORDER BY id
`
	)

	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		var requestStruct listMessagesRequest
		bodyBytes, _ := ioutil.ReadAll(r.Body)
		r.Body.Close()
		json.Unmarshal(bodyBytes, &requestStruct)

		if requestStruct.Limit == 0 {
			requestStruct.Limit = 100
		}

		messages := []listMessagesResponseMessage{}
		rows, _ := s.db.Query(r.Context(), listMessagesQueryString, requestStruct.Recipient, requestStruct.Start, requestStruct.Limit)
		for rows.Next() {
			var messageID int64
			var senderID int64
			var recipientID int64
			var timestamp time.Time
			var content map[string]interface{}

			rows.Scan(&messageID, &senderID, &recipientID, &timestamp, &content)
			messages = append(messages, listMessagesResponseMessage{
				ID:        messageID,
				Sender:    senderID,
				Recipient: recipientID,
				Timestamp: timestamp,
				Content:   content,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(listMessagesResponse{
			Messages: messages,
		})

		duration.Observe(time.Since(startTime).Seconds())
	}
}
