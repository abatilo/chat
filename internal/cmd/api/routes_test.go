package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/abatilo/multiregion-chat-experiment/internal/cmd/api"
	"github.com/google/uuid"
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

func Test_createUser_FailToInsert(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	mock, _ := pgxmock.NewConn()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO chat_user").WillReturnError(fmt.Errorf("couldn't insert"))
	mock.ExpectRollback()

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

	testRequest := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(testRequestBytes))

	// Act
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, testRequest)
	resp := w.Result()

	// Assert
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
}

func Test_login_Success(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	mock, _ := pgxmock.NewConn()

	mockPassword := "hunter2"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(mockPassword), bcrypt.MinCost)

	mock.ExpectQuery("SELECT id, password from chat_user").WillReturnRows(
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
