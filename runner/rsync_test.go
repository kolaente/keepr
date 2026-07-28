package runner

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"keepr/config"
)

func TestBuildRsyncArgs_Local(t *testing.T) {
	server := config.Server{
		Name: "local-backup",
		Type: "local",
	}
	path := config.Path{
		Remote: "/source/data",
		Local:  "/backups/data",
	}

	args := BuildRsyncArgs(server, path, path.Local, "")

	// Should have basic flags
	hasAVZ := false
	for _, arg := range args {
		if arg == "-avz" {
			hasAVZ = true
		}
	}
	if !hasAVZ {
		t.Error("Expected -avz flag")
	}

	// Last two args should be source and dest
	if len(args) < 2 {
		t.Fatal("Expected at least 2 args")
	}
	source := args[len(args)-2]
	dest := args[len(args)-1]

	if source != "/source/data" {
		t.Errorf("Source = %q, want /source/data", source)
	}
	if dest != "/backups/data" {
		t.Errorf("Dest = %q, want /backups/data", dest)
	}
}

func TestBuildRsyncArgs_Remote(t *testing.T) {
	server := config.Server{
		Name: "remote-server",
		Type: "remote",
		Host: "example.com",
		Port: 22,
		User: "backup",
		Key:  "/home/user/.ssh/id_rsa",
	}
	path := config.Path{
		Remote:    "/var/data",
		Local:     "/backups/server/data",
		BackupDir: "/backups/server/data.old",
	}

	args := BuildRsyncArgs(server, path, path.Local, path.BackupDir)

	// Check for SSH option
	hasSSH := false
	for i, arg := range args {
		if arg == "-e" && i+1 < len(args) {
			hasSSH = true
			sshCmd := args[i+1]
			expected := "ssh -p 22 -o StrictHostKeyChecking=accept-new -o BatchMode=yes -i /home/user/.ssh/id_rsa"
			if sshCmd != expected {
				t.Errorf("SSH command = %q, want %q", sshCmd, expected)
			}
		}
	}
	if !hasSSH {
		t.Error("Expected -e flag with SSH command")
	}

	// Check for --delete flag
	hasDelete := false
	for _, arg := range args {
		if arg == "--delete" {
			hasDelete = true
		}
	}
	if !hasDelete {
		t.Error("Expected --delete flag")
	}

	// Check for backup-dir flag
	hasBackupDir := false
	for i, arg := range args {
		if arg == "-b" && i+1 < len(args) && args[i+1] == "--backup-dir=/backups/server/data.old" {
			hasBackupDir = true
		}
	}
	if !hasBackupDir {
		t.Error("Expected -b --backup-dir flag")
	}

	// Last two args should be source (user@host:path) and dest
	if len(args) < 2 {
		t.Fatal("Expected at least 2 args")
	}
	source := args[len(args)-2]
	dest := args[len(args)-1]

	if source != "backup@example.com:/var/data" {
		t.Errorf("Source = %q, want backup@example.com:/var/data", source)
	}
	if dest != "/backups/server/data" {
		t.Errorf("Dest = %q, want /backups/server/data", dest)
	}
}

func TestCheckRsyncError_ExitCode24IsSuccess(t *testing.T) {
	// Exit code 24 means "some files vanished before they could be transferred"
	// This is common and should be treated as success
	err := checkRsyncError(&exec.ExitError{ProcessState: nil}, 24)
	if err != nil {
		t.Errorf("Exit code 24 should be success, got error: %v", err)
	}
}

func TestCheckRsyncError_ExitCode0IsSuccess(t *testing.T) {
	err := checkRsyncError(nil, 0)
	if err != nil {
		t.Errorf("Exit code 0 should be success, got error: %v", err)
	}
}

func TestCheckRsyncError_OtherExitCodesAreErrors(t *testing.T) {
	testErr := fmt.Errorf("process exited with code 1")
	err := checkRsyncError(testErr, 1)
	if err == nil {
		t.Error("Exit code 1 should be an error")
	}
}

func TestBuildRsyncArgs_BackupDirInsideDestinationExcluded(t *testing.T) {
	// A backup dir inside the destination would be deleted by --delete
	// and back up into itself, so it must be excluded (anchored)
	server := config.Server{
		Name: "remote-server",
		Type: "remote",
		Host: "example.com",
		Port: 22,
		User: "backup",
	}
	path := config.Path{
		Remote:    "/var/data",
		Local:     "/backups/server/data",
		BackupDir: "/backups/server/data/data_old",
	}

	args := BuildRsyncArgs(server, path, path.Local, path.BackupDir)

	hasExclude := false
	for _, arg := range args {
		if arg == "--exclude=/data_old" {
			hasExclude = true
		}
	}
	if !hasExclude {
		t.Errorf("Expected --exclude=/data_old for backup dir inside destination, got args: %v", args)
	}
}

func TestBuildRsyncArgs_BackupDirOutsideDestinationNotExcluded(t *testing.T) {
	server := config.Server{
		Name: "remote-server",
		Type: "remote",
		Host: "example.com",
		Port: 22,
		User: "backup",
	}
	path := config.Path{
		Remote:    "/var/data",
		Local:     "/backups/server/data",
		BackupDir: "/backups/server/data_old",
	}

	args := BuildRsyncArgs(server, path, path.Local, path.BackupDir)

	for _, arg := range args {
		if strings.HasPrefix(arg, "--exclude=") {
			t.Errorf("Should not add exclude for backup dir outside destination, got %s", arg)
		}
	}
}

func TestResolvePath(t *testing.T) {
	if got := ResolvePath("/backups", "server/data_old"); got != "/backups/server/data_old" {
		t.Errorf("ResolvePath relative = %q", got)
	}
	if got := ResolvePath("/backups", "/elsewhere/data_old"); got != "/elsewhere/data_old" {
		t.Errorf("ResolvePath absolute = %q", got)
	}
	if got := ResolvePath("/backups", ""); got != "" {
		t.Errorf("ResolvePath empty = %q", got)
	}
}
