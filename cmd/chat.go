package main

import (
	"os"
	"strings"

	"github.com/abatilo/chat/cmd/api"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	// Correctly set GOMAXPROCS in containerized environments
	_ "go.uber.org/automaxprocs"
)

func main() {

	// All viper config will be automatically sourced from environment variables
	// that start with CHAT_ and match a flag name
	viper.SetEnvPrefix("CHAT")
	// Let flags be named "--like-this" but environment variable would be
	// "LIKE_THIS"
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	rootCmd := &cobra.Command{
		Use:   "chat",
		Short: "This command is the entrypoint for all parts of the chat application",
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	rootCmd.AddCommand(api.Cmd(logger))
	rootCmd.Execute()
}
