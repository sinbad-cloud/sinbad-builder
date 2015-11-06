package main

import "os/exec"

// Exec runs the bash command
func Exec(dir, command string, arg ...string) ([]byte, error) {
	cmd := exec.Command(command, arg...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.CombinedOutput()
}
