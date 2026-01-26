package main

import (
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
	"keepr/preflight"
	"keepr/runner"
	"keepr/scheduler"
	"keepr/state"
	"keepr/web"
)

var configPath string
var runAll bool
var skipPreflight bool
var checkConnectivity bool

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
	Use:          "run [server-name]",
	Short:        "Run backup manually",
	Long:         "Run backup for a specific server or all servers",
	SilenceUsage: true,
	RunE:         runBackup,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show configured servers and their schedules",
	RunE:  showStatus,
}

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Run preflight checks on configuration",
	Long:  "Validates configuration, checks permissions, SSH keys, and optionally tests connectivity",
	RunE:  runPreflight,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "/etc/keepr/config.yaml", "config file path")
	runCmd.Flags().BoolVarP(&runAll, "all", "a", false, "run backup for all servers")
	serveCmd.Flags().BoolVar(&skipPreflight, "skip-preflight", false, "Skip preflight checks (not recommended)")
	preflightCmd.Flags().BoolVar(&checkConnectivity, "check-connectivity", false, "Also check SSH connectivity to remote servers")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(preflightCmd)
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

	// Run preflight checks (unless skipped)
	if !skipPreflight {
		fmt.Println("Running preflight checks...")
		if errors := preflight.RunAll(cfg); len(errors) > 0 {
			fmt.Println("Preflight checks failed:")
			for _, err := range errors {
				fmt.Printf("  ✗ %v\n", err)
			}
			return fmt.Errorf("preflight checks failed with %d error(s)", len(errors))
		}
		fmt.Println("Preflight checks passed ✓")
	} else {
		fmt.Println("Skipping preflight checks (--skip-preflight)")
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
		// Trigger backup via API
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/run/"+name, nil)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", name, err)
			failed = append(failed, name)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+cfg.Web.APISecret)

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("  ✗ %s: failed to connect to server: %v\n", name, err)
			fmt.Println("    Is the keepr server running? (keepr serve)")
			failed = append(failed, name)
			continue
		}

		if resp.StatusCode != http.StatusAccepted {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			fmt.Printf("  ✗ %s: %s\n", name, string(body))
			failed = append(failed, name)
			continue
		}
		_ = resp.Body.Close()

		fmt.Printf("  ✓ %s: scheduled\n", name)
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to schedule: %v", failed)
	}

	return nil
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

func runPreflight(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("Running preflight checks...")
	fmt.Println()

	var errors []error
	if checkConnectivity {
		errors = preflight.RunAllWithConnectivity(cfg)
	} else {
		errors = preflight.RunAll(cfg)
	}

	if len(errors) > 0 {
		fmt.Println("✗ Preflight checks failed:")
		fmt.Println()
		for _, err := range errors {
			fmt.Printf("  • %v\n", err)
		}
		fmt.Println()
		fmt.Printf("Total: %d error(s)\n", len(errors))
		return fmt.Errorf("preflight failed")
	}

	fmt.Println("✓ All preflight checks passed")
	if !checkConnectivity {
		fmt.Println()
		fmt.Println("Tip: Run with --check-connectivity to also verify SSH connections")
	}
	return nil
}
