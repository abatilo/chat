package api

import (
	"github.com/abatilo/multiregion-chat-experiment/internal/cmd/api"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// Cmd is the main command for the API package
func Cmd(logger zerolog.Logger) *cobra.Command {
	return api.Cmd(logger)
}
