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

		if req.Content.Width == 0 {
			req.Content.Width = 64
		}
		if req.Content.Height == 0 {
			req.Content.Height = 64
		}

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
			Timestamp: createdAt.UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)

		duration.Observe(time.Since(startTime).Seconds())
	}
}

func (s *Server) listMessages() http.HandlerFunc {
	log := s.logger.With().Str("method", "listMessages").Logger()

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
		ID        int64           `json:"id"`
		Sender    int64           `json:"sender"`
		Recipient int64           `json:"recipient"`
		Timestamp time.Time       `json:"timestamp"`
		Content   json.RawMessage `json:"content"`
	}

	type listMessagesResponse struct {
		Messages []listMessagesResponseMessage `json:"messages"`
	}

	const (
		listMessageIDsQueryString = `
SELECT id
	FROM message
	WHERE recipient_id = $1 AND id >= $2
	ORDER BY id
	LIMIT $3
`
		countMessagesByTypeQueryString = `
SELECT message_type.name, COUNT(*)
	FROM message
	JOIN message_type ON (message.message_type_id = message_type.id)
	GROUP BY message_type.name
`

		listTextMessagesQueryString = `
SELECT message.id, message.sender_id, message.recipient_id, message.created_at,
			 message_type.name,
			 text_message.text
	FROM message
		JOIN message_type ON (message.message_type_id = message_type.id)
		JOIN text_message ON (message.id = text_message.message_id)
	WHERE
		message.recipient_id = $1
		AND message.id >= $2
	ORDER BY message.id
	LIMIT $3
`

		listImageMessagesQueryString = `
SELECT message.id, message.sender_id, message.recipient_id, message.created_at,
			 message_type.name,
			 image_message.url, image_message.width, image_message.height
	FROM message
		JOIN message_type ON (message.message_type_id = message_type.id)
		JOIN image_message ON (message.id = image_message.message_id)
	WHERE
		message.recipient_id = $1
		AND message.id >= $2
	ORDER BY message.id
	LIMIT $3
`

		listVideoMessagesQueryString = `
SELECT message.id, message.sender_id, message.recipient_id, message.created_at,
			 message_type.name,
			 video_message.url,
			 video_source.name
	FROM message
		JOIN message_type ON (message.message_type_id = message_type.id)
		JOIN video_message ON (message.id = video_message.message_id)
		JOIN video_source ON (video_message.source = video_source.id)
	WHERE
		message.recipient_id = $1
		AND message.id >= $2
	ORDER BY message.id
	LIMIT $3
`
	)

	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			err := fmt.Errorf("Couldn't read request body: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		var requestStruct listMessagesRequest
		err = json.Unmarshal(bodyBytes, &requestStruct)
		if err != nil {
			err := fmt.Errorf("Couldn't unmarshal request body: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		if requestStruct.Limit == 0 {
			requestStruct.Limit = 100
		}

		tx, err := s.db.Begin(r.Context())
		if err != nil {
			err := fmt.Errorf("Couldn't begin transaction: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			tx.Rollback(r.Context())
			return
		}

		messageIDs := []int64{}
		messageTypeToCount := make(map[string]int64)
		messageIDToResponseStruct := make(map[int64]listMessagesResponseMessage)

		messageIDRows, err := tx.Query(r.Context(), listMessageIDsQueryString, requestStruct.Recipient, requestStruct.Start, requestStruct.Limit)
		if err != nil {
			err := fmt.Errorf("Couldn't query for message IDs: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			tx.Rollback(r.Context())
			return
		}
		for messageIDRows.Next() {
			var messageID int64
			messageIDRows.Scan(&messageID)
			messageIDs = append(messageIDs, messageID)
		}

		messageTypeCountRows, err := tx.Query(r.Context(), countMessagesByTypeQueryString)
		if err != nil {
			err := fmt.Errorf("Couldn't query for message type counts: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			tx.Rollback(r.Context())
			return
		}
		for messageTypeCountRows.Next() {
			var messageType string
			var messageTypeCount int64
			messageTypeCountRows.Scan(&messageType, &messageTypeCount)
			messageTypeToCount[messageType] = messageTypeCount
		}

		textMessageRows, err := tx.Query(r.Context(), listTextMessagesQueryString, requestStruct.Recipient, requestStruct.Start, messageTypeToCount["text"])
		if err != nil {
			err := fmt.Errorf("Couldn't query for text messages: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			tx.Rollback(r.Context())
			return
		}
		for textMessageRows.Next() {
			var messageID int64
			var senderID int64
			var recipientID int64
			var timestamp time.Time
			var messageType string
			var messageText string

			textMessageRows.Scan(&messageID, &senderID, &recipientID, &timestamp, &messageType, &messageText)

			content := struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				Type: messageType,
				Text: messageText,
			}
			contentBytes, _ := json.Marshal(content)
			singleResponse := listMessagesResponseMessage{
				ID:        messageID,
				Sender:    senderID,
				Recipient: recipientID,
				Timestamp: timestamp,
				Content:   contentBytes,
			}
			log.Info().Msgf("%#v", singleResponse)
			messageIDToResponseStruct[messageID] = singleResponse
		}

		imageMessageRows, err := tx.Query(r.Context(), listImageMessagesQueryString, requestStruct.Recipient, requestStruct.Start, messageTypeToCount["image"])
		if err != nil {
			err := fmt.Errorf("Couldn't query for image messages: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			tx.Rollback(r.Context())
			return
		}
		for imageMessageRows.Next() {
			var messageID int64
			var senderID int64
			var recipientID int64
			var timestamp time.Time
			var messageType string
			var url string
			var width int16
			var height int16

			imageMessageRows.Scan(&messageID, &senderID, &recipientID, &timestamp, &messageType, &url, &width, &height)

			content := struct {
				Type   string `json:"type"`
				URL    string `json:"url"`
				Width  int16  `json:"width"`
				Height int16  `json:"height"`
			}{
				Type:   messageType,
				URL:    url,
				Width:  width,
				Height: height,
			}
			contentBytes, _ := json.Marshal(content)
			singleResponse := listMessagesResponseMessage{
				ID:        messageID,
				Sender:    senderID,
				Recipient: recipientID,
				Timestamp: timestamp,
				Content:   contentBytes,
			}
			log.Info().Msgf("%#v", singleResponse)
			messageIDToResponseStruct[messageID] = singleResponse
		}

		videoMessageRows, err := tx.Query(r.Context(), listVideoMessagesQueryString, requestStruct.Recipient, requestStruct.Start, messageTypeToCount["video"])
		if err != nil {
			err := fmt.Errorf("Couldn't query for video messages: %w", err)
			log.Err(err).Msg(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			tx.Rollback(r.Context())
			return
		}
		for videoMessageRows.Next() {
			var messageID int64
			var senderID int64
			var recipientID int64
			var timestamp time.Time
			var messageType string
			var url string
			var source string

			videoMessageRows.Scan(&messageID, &senderID, &recipientID, &timestamp, &messageType, &url, &source)

			content := struct {
				Type   string `json:"type"`
				URL    string `json:"url"`
				Source string `json:"source"`
			}{
				Type:   messageType,
				URL:    url,
				Source: source,
			}
			contentBytes, _ := json.Marshal(content)
			singleResponse := listMessagesResponseMessage{
				ID:        messageID,
				Sender:    senderID,
				Recipient: recipientID,
				Timestamp: timestamp,
				Content:   contentBytes,
			}
			log.Info().Msgf("%#v", singleResponse)
			messageIDToResponseStruct[messageID] = singleResponse
		}

		messages := []listMessagesResponseMessage{}
		for _, messageID := range messageIDs {
			if message, found := messageIDToResponseStruct[messageID]; found {
				messages = append(messages, message)
			}
		}

		tx.Commit(r.Context())
		json.NewEncoder(w).Encode(listMessagesResponse{
			Messages: messages,
		})
		duration.Observe(time.Since(startTime).Seconds())
	}
}
