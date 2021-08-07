package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/abatilo/multiregion-chat-experiment/internal/metrics"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func run(logger zerolog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the API server",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := &ServerConfig{
				Port:      viper.GetInt(FlagPortName),
				AdminPort: viper.GetInt(FlagAdminPortName),
			}
			logger.Info().Msgf("%#v", cfg)

			// Build dependendies
			// End build dependendies

			s := NewServer(cfg,
				WithLogger(logger),
				WithMetrics(&metrics.PrometheusMetrics{}),
			)

			// Register signal handlers for graceful shutdown
			done := make(chan struct{})
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-quit
				logger.Info().Msg("Shutting down gracefully")
				s.Shutdown(context.Background())
				close(done)
			}()

			if err := s.Start(); err != http.ErrServerClosed {
				logger.Error().Err(err).Msg("couldn't shut down gracefully")
			}
			<-done
			logger.Info().Msg("Exiting")
		},
	}

	cmd.PersistentFlags().Int(FlagPortName, 8080, "The port to run the web server on")
	viper.BindPFlag(FlagPortName, cmd.PersistentFlags().Lookup(FlagPortName))

	cmd.PersistentFlags().Int(FlagAdminPortName, 8081, "The admin port to run the administrative web server on")
	viper.BindPFlag(FlagAdminPortName, cmd.PersistentFlags().Lookup(FlagAdminPortName))

	return cmd
}

func migrate(logger zerolog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Execute database migrations for the API server",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info().Msg("Running migrations for api server")
		},
	}
}

// Cmd parses config and starts the application
func Cmd(logger zerolog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Runs the api web server",
	}

	cmd.AddCommand(run(logger), migrate(logger))

	return cmd
}
