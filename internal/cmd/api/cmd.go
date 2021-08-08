package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abatilo/chat/internal/metrics"
	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	mg "github.com/golang-migrate/migrate/v4"

	// Import postgres driver for golang-migrate
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	// Import file driver for golang-migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/jackc/pgx/v4/pgxpool"
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
			db, err := pgxpool.Connect(context.Background(), "postgres://postgres:localdev@postgresql:5432/postgres?sslmode=disable")
			if err != nil {
				logger.Panic().Err(err).Msg("Unable to connect to postgres")
			}

			sessionManager := scs.New()
			sessionManager.Store = pgxstore.New(db)
			sessionManager.Lifetime = 12 * time.Hour
			sessionManager.IdleTimeout = 3 * time.Hour
			// End build dependendies

			s := NewServer(cfg,
				WithLogger(logger),
				WithMetrics(&metrics.PrometheusMetrics{}),
				WithDB(db),
				WithSessionManager(sessionManager),
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
		Use:       "migrate (up|down)",
		Short:     "Execute database migrations for the API server",
		ValidArgs: []string{"up", "down"},
		Args:      cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info().Msg("Running migrations for api server")
			m, err := mg.New(
				"file:///app/db/migrations",
				"postgres://postgres:localdev@postgresql:5432/postgres?sslmode=disable")
			if err != nil {
				logger.Panic().Err(err).Msg("Couldn't instantiate new migration")
			}

			if args[0] == "up" {
				logger.Info().Msg("Running up migrations")
				err = m.Up()
				if err != nil && err != mg.ErrNoChange {
					logger.Panic().Err(err).Msg("Couldn't run up")
				}
				logger.Info().Msg("Up migrations were ran successfully")
			} else if args[0] == "down" {
				logger.Info().Msg("Running down migrations")
				err = m.Down()
				if err != nil && err != mg.ErrNoChange {
					logger.Panic().Err(err).Msg("Couldn't run down")
				}
				logger.Info().Msg("Down migrations were ran successfully")
			}
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
