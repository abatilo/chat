package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/abatilo/chat/internal/cmd/api"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/pashagolub/pgxmock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func Test_check(t *testing.T) {
	assert := assert.New(t)

	srv := api.NewServer(
		&api.ServerConfig{},
		api.WithAdminServer(&http.Server{}),
	)

	w := httptest.NewRecorder()
	testRequest := httptest.NewRequest(http.MethodGet, "/check", nil)
	srv.ServeHTTP(w, testRequest)

	resp := w.Result()
	assert.Equal(http.StatusOK, resp.StatusCode)
}

func Test_createUser_Success(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	mock, _ := pgxmock.NewConn()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO chat_user").WillReturnRows(
		pgxmock.NewRows([]string{"id"}).AddRow(int64(17)))
	mock.ExpectCommit()

	srv := api.NewServer(
		&api.ServerConfig{},
		api.WithAdminServer(&http.Server{}),
		api.WithDB(mock),
	)

	testRequestStruct := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: "fakeuser",
		Password: "fakepassword",
	}
	testRequestBytes, _ := json.Marshal(&testRequestStruct)

	type testResponseStruct struct {
		ID int64 `json:"id"`
	}
	expectedResponseStruct := testResponseStruct{
		ID: 17,
	}
	actualResponseStruct := testResponseStruct{}
	testRequest := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(testRequestBytes))

	// Act
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, testRequest)
	resp := w.Result()
	json.NewDecoder(resp.Body).Decode(&actualResponseStruct)

	// Assert
	assert.Equal(http.StatusCreated, resp.StatusCode)
	assert.Equal("application/json", resp.Header.Get("Content-Type"))
	assert.Equal(expectedResponseStruct, actualResponseStruct)
}

func Test_login_Success(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	mock, _ := pgxmock.NewConn()

	mockPassword := "hunter2"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(mockPassword), bcrypt.MinCost)

	mock.ExpectQuery("SELECT id, password FROM chat_user").WillReturnRows(
		pgxmock.NewRows([]string{"id", "password"}).AddRow(int64(1), hashedPassword))

	srv := api.NewServer(
		&api.ServerConfig{},
		api.WithAdminServer(&http.Server{}),
		api.WithDB(mock),
		api.WithLogger(zerolog.New(os.Stdout)),
	)

	testRequestStruct := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: "fakeuser",
		Password: mockPassword,
	}
	testRequestBytes, _ := json.Marshal(&testRequestStruct)

	type testResponseStruct struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	actualResponseStruct := testResponseStruct{}
	testRequest := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(testRequestBytes))

	// Act
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, testRequest)
	resp := w.Result()
	json.NewDecoder(resp.Body).Decode(&actualResponseStruct)

	// Assert
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("application/json", resp.Header.Get("Content-Type"))
	assert.NotPanics(func() {
		uuid.MustParse(actualResponseStruct.Token)
	})
}

func Test_createMessage_TextMessageSuccess(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	mock, _ := pgxmock.NewConn()

	mockPassword := "hunter2"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(mockPassword), bcrypt.MinCost)

	mock.ExpectQuery("SELECT id, password FROM chat_user").WillReturnRows(
		pgxmock.NewRows([]string{"id", "password"}).AddRow(int64(1), hashedPassword))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, name FROM message_type").WillReturnRows(
		pgxmock.NewRows([]string{"id", "name"}).AddRow(int16(1), "text"))
	mock.ExpectQuery("INSERT INTO message").WillReturnRows(
		pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(1), time.Now()))
	mock.ExpectExec("INSERT INTO text_message").WillReturnResult(pgconn.CommandTag{})
	mock.ExpectCommit()

	srv := api.NewServer(
		&api.ServerConfig{},
		api.WithAdminServer(&http.Server{}),
		api.WithDB(mock),
		api.WithLogger(zerolog.New(os.Stdout)),
	)

	loginRequestStruct := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: "fakeuser",
		Password: mockPassword,
	}
	loginRequestBytes, _ := json.Marshal(&loginRequestStruct)

	type loginResponseStruct struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	actualLoginResponseStruct := loginResponseStruct{}
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginRequestBytes))

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, loginRequest)
	resp := w.Result()
	json.NewDecoder(resp.Body).Decode(&actualLoginResponseStruct)

	token := actualLoginResponseStruct.Token

	type contentStruct struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	createTextMessageRequestStruct := struct {
		Sender    int64         `json:"sender"`
		Recipient int64         `json:"recipient"`
		Content   contentStruct `json:"content"`
	}{
		Sender:    1,
		Recipient: 1,
		Content: contentStruct{
			Type: "text",
			Text: "testing",
		},
	}

	createTextMessageRequestBytes, _ := json.Marshal(createTextMessageRequestStruct)
	createTextMessageRequest := httptest.NewRequest(http.MethodPost, "/messages", bytes.NewReader(createTextMessageRequestBytes))
	createTextMessageResponseStruct := struct {
		ID        int64  `json:"id"`
		Timestamp string `json:"timestamp"`
	}{}
	createTextMessageRequest.Header.Set("authorization", token)
	createTextMessageRequest.AddCookie(resp.Cookies()[0])

	// Act
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, createTextMessageRequest)
	resp = w.Result()
	json.NewDecoder(resp.Body).Decode(&createTextMessageResponseStruct)

	// Assert

	assert.Equal(http.StatusCreated, resp.StatusCode)
	assert.Equal("application/json", resp.Header.Get("Content-Type"))
}

func Test_createMessage_ImageMessageSuccess(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	mock, _ := pgxmock.NewConn()

	mockPassword := "hunter2"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(mockPassword), bcrypt.MinCost)

	mock.ExpectQuery("SELECT id, password FROM chat_user").WillReturnRows(
		pgxmock.NewRows([]string{"id", "password"}).AddRow(int64(1), hashedPassword))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, name FROM message_type").WillReturnRows(
		pgxmock.NewRows([]string{"id", "name"}).AddRow(int16(1), "image"))
	mock.ExpectQuery("INSERT INTO message").WillReturnRows(
		pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(1), time.Now()))
	mock.ExpectExec("INSERT INTO image_message").WillReturnResult(pgconn.CommandTag{})
	mock.ExpectCommit()

	srv := api.NewServer(
		&api.ServerConfig{},
		api.WithAdminServer(&http.Server{}),
		api.WithDB(mock),
		api.WithLogger(zerolog.New(os.Stdout)),
	)

	loginRequestStruct := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: "fakeuser",
		Password: mockPassword,
	}
	loginRequestBytes, _ := json.Marshal(&loginRequestStruct)

	type loginResponseStruct struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	actualLoginResponseStruct := loginResponseStruct{}
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginRequestBytes))

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, loginRequest)
	resp := w.Result()
	json.NewDecoder(resp.Body).Decode(&actualLoginResponseStruct)

	token := actualLoginResponseStruct.Token

	type contentStruct struct {
		Type string `json:"type"`
		URL  string `json:"text"`
	}
	createTextMessageRequestStruct := struct {
		Sender    int64         `json:"sender"`
		Recipient int64         `json:"recipient"`
		Content   contentStruct `json:"content"`
	}{
		Sender:    1,
		Recipient: 1,
		Content: contentStruct{
			Type: "image",
			URL:  "testing",
		},
	}

	createTextMessageRequestBytes, _ := json.Marshal(createTextMessageRequestStruct)
	createTextMessageRequest := httptest.NewRequest(http.MethodPost, "/messages", bytes.NewReader(createTextMessageRequestBytes))
	createTextMessageResponseStruct := struct {
		ID        int64  `json:"id"`
		Timestamp string `json:"timestamp"`
	}{}
	createTextMessageRequest.Header.Set("authorization", token)
	createTextMessageRequest.AddCookie(resp.Cookies()[0])

	// Act
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, createTextMessageRequest)
	resp = w.Result()
	json.NewDecoder(resp.Body).Decode(&createTextMessageResponseStruct)

	// Assert

	assert.Equal(http.StatusCreated, resp.StatusCode)
	assert.Equal("application/json", resp.Header.Get("Content-Type"))
}

func Test_createMessage_VideoMessageSuccess(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	mock, _ := pgxmock.NewConn()

	mockPassword := "hunter2"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(mockPassword), bcrypt.MinCost)

	mock.ExpectQuery("SELECT id, password FROM chat_user").WillReturnRows(
		pgxmock.NewRows([]string{"id", "password"}).AddRow(int64(1), hashedPassword))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, name FROM message_type").WillReturnRows(
		pgxmock.NewRows([]string{"id", "name"}).AddRow(int16(1), "video"))
	mock.ExpectQuery("INSERT INTO message").WillReturnRows(
		pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(1), time.Now()))
	mock.ExpectQuery("SELECT id, name FROM video_source").WillReturnRows(
		pgxmock.NewRows([]string{"id", "name"}).AddRow(int16(1), "youtube"))
	mock.ExpectExec("INSERT INTO video_message").WillReturnResult(pgconn.CommandTag{})
	mock.ExpectCommit()

	srv := api.NewServer(
		&api.ServerConfig{},
		api.WithAdminServer(&http.Server{}),
		api.WithDB(mock),
		api.WithLogger(zerolog.New(os.Stdout)),
	)

	loginRequestStruct := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: "fakeuser",
		Password: mockPassword,
	}
	loginRequestBytes, _ := json.Marshal(&loginRequestStruct)

	type loginResponseStruct struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	actualLoginResponseStruct := loginResponseStruct{}
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginRequestBytes))

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, loginRequest)
	resp := w.Result()
	json.NewDecoder(resp.Body).Decode(&actualLoginResponseStruct)

	token := actualLoginResponseStruct.Token

	type contentStruct struct {
		Type   string `json:"type"`
		URL    string `json:"text"`
		Source string `json:"source"`
	}
	createTextMessageRequestStruct := struct {
		Sender    int64         `json:"sender"`
		Recipient int64         `json:"recipient"`
		Content   contentStruct `json:"content"`
	}{
		Sender:    1,
		Recipient: 1,
		Content: contentStruct{
			Type:   "video",
			URL:    "testing",
			Source: "youtube",
		},
	}

	createTextMessageRequestBytes, _ := json.Marshal(createTextMessageRequestStruct)
	createTextMessageRequest := httptest.NewRequest(http.MethodPost, "/messages", bytes.NewReader(createTextMessageRequestBytes))
	createTextMessageResponseStruct := struct {
		ID        int64  `json:"id"`
		Timestamp string `json:"timestamp"`
	}{}
	createTextMessageRequest.Header.Set("authorization", token)
	createTextMessageRequest.AddCookie(resp.Cookies()[0])

	// Act
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, createTextMessageRequest)
	resp = w.Result()
	json.NewDecoder(resp.Body).Decode(&createTextMessageResponseStruct)

	// Assert

	assert.Equal(http.StatusCreated, resp.StatusCode)
	assert.Equal("application/json", resp.Header.Get("Content-Type"))
}
