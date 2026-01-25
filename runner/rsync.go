package runner

import (
	"fmt"

	"keepr/config"
)

// BuildRsyncArgs builds the rsync command arguments for a given server and path.
func BuildRsyncArgs(server config.Server, path config.Path) []string {
	args := []string{"-avz"}

	// For remote servers, add SSH options
	if server.Type == "remote" {
		sshCmd := fmt.Sprintf("ssh -p %d", server.Port)
		if server.Key != "" {
			sshCmd += " -i " + server.Key
		}
		args = append(args, "-e", sshCmd)
		args = append(args, "--delete")
	}

	// Handle backup directory option
	if path.BackupDir != "" {
		args = append(args, "-b", "--backup-dir="+path.BackupDir)
	}

	// Build source path
	var source string
	if server.Type == "remote" {
		source = fmt.Sprintf("%s@%s:%s", server.User, server.Host, path.Remote)
	} else {
		source = path.Remote
	}

	args = append(args, source, path.Local)

	return args
}
