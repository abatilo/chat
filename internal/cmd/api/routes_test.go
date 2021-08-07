package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abatilo/multiregion-chat-experiment/internal/cmd/api"
	"github.com/stretchr/testify/assert"
)

func Test_ping(t *testing.T) {
	assert := assert.New(t)

	srv := api.NewServer(
		&api.ServerConfig{},
		api.WithAdminServer(&http.Server{}),
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	srv.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(http.StatusOK, resp.StatusCode)
}
