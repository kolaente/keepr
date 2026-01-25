package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"keepr/config"
	"keepr/runner"
	"keepr/scheduler"
	"keepr/state"
	"keepr/web"
)

var configPath string
var runAll bool

var rootCmd = &cobra.Command{
	Use:   "keepr",
	Short: "Keepr - A backup service with web UI",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the backup service",
	RunE:  runServe,
}

var runCmd = &cobra.Command{
	Use:   "run [server-name]",
	Short: "Run backup manually",
	Long:  "Run backup for a specific server or all servers",
	RunE:  runBackup,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show configured servers and their schedules",
	RunE:  showStatus,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "/etc/keepr/config.yaml", "config file path")
	runCmd.Flags().BoolVarP(&runAll, "all", "a", false, "run backup for all servers")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
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
	webServer := web.New(sm, cfg, r)

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
	_ = httpServer.Close()
	log.Println("Goodbye!")

	return nil
}

func runBackup(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check API secret is configured
	if cfg.Web.APISecret == "" {
		return fmt.Errorf("api_secret is required in config for run command")
	}

	// Determine which servers to run
	var serverNames []string

	if runAll {
		for _, s := range cfg.Servers {
			serverNames = append(serverNames, s.Name)
		}
	} else if len(args) > 0 {
		serverName := args[0]
		found := false
		for _, s := range cfg.Servers {
			if s.Name == serverName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("server not found: %s", serverName)
		}
		serverNames = append(serverNames, serverName)
	} else {
		return fmt.Errorf("specify a server name or use --all")
	}

	// Determine API base URL
	listen := cfg.Web.Listen
	if listen == "" {
		listen = ":8080"
	}
	// Convert listen address to URL
	baseURL := fmt.Sprintf("http://localhost%s", listen)
	if listen[0] != ':' {
		baseURL = fmt.Sprintf("http://%s", listen)
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Trigger backups
	var failed []string
	for _, name := range serverNames {
		fmt.Printf("Running backup for %s...\n", name)

		// Trigger backup via API
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/run/"+name, nil)
		if err != nil {
			fmt.Printf("  ✗ Failed: %v\n", err)
			failed = append(failed, name)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+cfg.Web.APISecret)

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("  ✗ Failed to connect to server: %v\n", err)
			fmt.Println("    Is the keepr server running? (keepr serve)")
			failed = append(failed, name)
			continue
		}

		if resp.StatusCode != http.StatusAccepted {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("  ✗ Failed: %s\n", string(body))
			failed = append(failed, name)
			continue
		}
		resp.Body.Close()

		// Poll for completion
		if err := waitForCompletion(client, baseURL, cfg.Web.APISecret, name); err != nil {
			fmt.Printf("  ✗ Failed: %v\n", err)
			failed = append(failed, name)
		} else {
			fmt.Printf("  ✓ Completed\n")
		}
	}

	// Print summary
	fmt.Println()
	if len(failed) > 0 {
		fmt.Printf("Failed: %d/%d servers\n", len(failed), len(serverNames))
		for _, name := range failed {
			fmt.Printf("  - %s\n", name)
		}
		return fmt.Errorf("some backups failed")
	}

	fmt.Printf("Success: %d/%d servers\n", len(serverNames), len(serverNames))
	return nil
}

func waitForCompletion(client *http.Client, baseURL, secret, name string) error {
	pollInterval := 2 * time.Second
	timeout := 2 * time.Hour
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/api/status/"+name, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+secret)

		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		var status struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			resp.Body.Close()
			return err
		}
		resp.Body.Close()

		switch status.Status {
		case "success":
			return nil
		case "failed_backup", "failed_pre_hook", "failed_post_hook":
			return fmt.Errorf("backup %s", status.Status)
		case "running":
			time.Sleep(pollInterval)
		default:
			// idle or unknown, keep polling briefly in case it hasn't started yet
			time.Sleep(pollInterval)
		}
	}

	return fmt.Errorf("timeout waiting for backup to complete")
}

func showStatus(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Print header
	fmt.Printf("%-20s %-10s %s\n", "SERVER", "TYPE", "SCHEDULE")
	fmt.Printf("%-20s %-10s %s\n", "------", "----", "--------")

	// Print each server
	for _, server := range cfg.Servers {
		schedule := server.Schedule
		if schedule == "" {
			schedule = "(no schedule)"
		}
		serverType := server.Type
		if serverType == "" {
			serverType = "remote"
		}
		fmt.Printf("%-20s %-10s %s\n", server.Name, serverType, schedule)
	}

	return nil
}
