package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"keepr/config"
	"keepr/runner"
	"keepr/scheduler"
	"keepr/state"
	"keepr/web"
)

var configPath string

var rootCmd = &cobra.Command{
	Use:   "keepr",
	Short: "Keepr - A backup service with web UI",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the backup service",
	RunE:  runServe,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "/etc/keepr/config.yaml", "config file path")
	rootCmd.AddCommand(serveCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log.Printf("Loaded config from %s", configPath)
	log.Printf("Backup base path: %s", cfg.BackupBasePath)
	log.Printf("Configured %d servers", len(cfg.Servers))

	// Create state manager
	sm := state.New()

	// Create runner
	r := runner.New(cfg, sm)

	// Create scheduler
	sched := scheduler.New(sm)

	// Add all servers to scheduler
	for _, server := range cfg.Servers {
		srv := server // capture for closure
		err := sched.Add(srv, func(s config.Server) {
			log.Printf("Starting scheduled backup for %s", s.Name)
			if err := r.Run(s); err != nil {
				log.Printf("Backup failed for %s: %v", s.Name, err)
			} else {
				log.Printf("Backup completed for %s", s.Name)
			}
		})
		if err != nil {
			return fmt.Errorf("failed to schedule %s: %w", srv.Name, err)
		}
		if srv.Schedule != "" {
			log.Printf("Scheduled %s with cron: %s", srv.Name, srv.Schedule)
		}
	}

	// Start scheduler
	sched.Start()
	log.Println("Scheduler started")

	// Create web server
	webServer := web.New(sm, cfg)

	// Determine listen address
	listen := cfg.Web.Listen
	if listen == "" {
		listen = ":8080"
	}

	// Start HTTP server in goroutine
	httpServer := &http.Server{
		Addr:    listen,
		Handler: webServer,
	}

	go func() {
		log.Printf("Web server listening on %s", listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	sched.Stop()
	httpServer.Close()
	log.Println("Goodbye!")

	return nil
}
