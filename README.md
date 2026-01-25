# Keepr

A Go backup service with YAML configuration, built-in scheduler, and web UI for monitoring.

## Features

- **Rsync-based backups** - Reliable file synchronization for local and remote servers
- **Cron scheduling** - Built-in scheduler with standard cron expressions
- **Web dashboard** - Real-time status monitoring with live log streaming
- **Pre/post hooks** - Run custom commands before and after backups
- **Retention management** - Automatic cleanup of old backup versions
- **Heartbeat support** - Notify external services on successful backups

## Installation

```bash
go build -o keepr .
sudo mv keepr /usr/local/bin/
```

## Usage

### Start the backup service

```bash
keepr serve --config /path/to/config.yaml
```

This starts the scheduler and web dashboard. Backups run automatically based on their cron schedules.

### Run backups manually

```bash
# Run backup for a specific server
keepr run myserver --config /path/to/config.yaml

# Run all backups
keepr run --all --config /path/to/config.yaml
```

### Check configured servers

```bash
keepr status --config /path/to/config.yaml
```

## Configuration

Create a `config.yaml` file (see `config.example.yaml` for a complete example):

```yaml
# Base path for all backups
backup_base_path: /backups

# Web dashboard settings
web:
  listen: ":8080"

# Default values for all servers
defaults:
  user: backup
  port: 22
  retention_days: 7
  schedule: "0 2 * * *"  # Daily at 2am

# Server configurations
servers:
  - name: webserver
    type: remote
    host: web.example.com
    key: /home/backup/.ssh/id_rsa
    paths:
      - remote: /var/www
        local: /backups/webserver/www

  - name: database
    type: local
    schedule: "0 */6 * * *"  # Every 6 hours
    pre_hook: "pg_dumpall > /tmp/db.sql"
    post_hook: "rm /tmp/db.sql"
    paths:
      - remote: /tmp
        local: /backups/database
```

### Configuration Options

#### Top Level

| Option | Description |
|--------|-------------|
| `backup_base_path` | Base directory for all backups (required) |
| `web.listen` | Web server listen address (default: `:8080`) |

#### Defaults / Server

| Option | Description |
|--------|-------------|
| `name` | Server identifier (required) |
| `type` | `remote` or `local` (default: `remote`) |
| `host` | Remote hostname (required for remote type) |
| `port` | SSH port (default: `22`) |
| `user` | SSH username |
| `key` | Path to SSH private key |
| `schedule` | Cron expression for automatic backups |
| `retention_days` | Days to keep old backup versions |
| `pre_hook` | Command to run before backup |
| `post_hook` | Command to run after backup |
| `heartbeat` | URL to call on successful backup |
| `paths` | List of paths to backup |

#### Path

| Option | Description |
|--------|-------------|
| `remote` | Source path (on remote or local filesystem) |
| `local` | Destination path for backup |
| `backup_dir` | Directory for incremental backup versions |

## Web Dashboard

Access the web dashboard at `http://localhost:8080` (or your configured address).

- **Dashboard** (`/`) - Overview of all servers with status, last run, and next scheduled run
- **Logs** (`/logs/{server}`) - Real-time log streaming for each server

## Development

### Run tests

```bash
# Unit tests
go test ./... -v

# Integration tests
go test -tags=integration -v ./...
```

### Project structure

```
keepr/
├── main.go              # CLI entry point (cobra)
├── config/              # Configuration loading and validation
├── state/               # State manager with log buffer
├── runner/              # Backup execution (rsync, hooks, cleanup)
├── scheduler/           # Cron-based job scheduling
├── web/                 # HTTP server and templates
└── integration_test.go  # Integration tests
```

## License

MIT
