// Package systemdutils provides utilities for executing commands via systemd
package systemdutils

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
)

// RunCommandAsRoot executes a command as root using systemd transient units
// It creates a oneshot service unit that executes the command and then starts it
func RunCommandAsRoot(command string) (string, error) {
	// Connect to systemd over D-Bus
	conn, err := systemdDbus.New()
	if err != nil {
		return "", fmt.Errorf("failed to connect to systemd: %w", err)
	}
	defer conn.Close()

	// Generate a unique unit name based on the command
	unitName := fmt.Sprintf("oneshot-cmd-%d.service", time.Now().UnixNano())

	// Define unit properties
	properties := []systemdDbus.Property{
		systemdDbus.PropDescription("Transient service for executing command"),
		systemdDbus.PropType("oneshot"),
		systemdDbus.PropExecStart([]string{"/bin/sh", "-c", command}, false),
		systemdDbus.PropRemainAfterExit(true),
	}

	// Start the transient unit
	ch := make(chan string)
	_, err = conn.StartTransientUnit(unitName, "replace", properties, ch)
	if err != nil {
		return "", fmt.Errorf("failed to start transient unit: %w", err)
	}

	// Wait for job to complete
	result := <-ch
	if result != "done" {
		return "", fmt.Errorf("failed to run command, job status: %s", result)
	}

	// Get unit properties to check for exit status
	prop, err := conn.GetUnitProperties(unitName)
	if err != nil {
		return "", fmt.Errorf("failed to get unit properties: %w", err)
	}

	// Get exit code
	exitCodeVal, ok := prop["ExecMainStatus"]
	if !ok {
		// If the property doesn't exist, assume success
		fmt.Println("Warning: ExecMainStatus property not found, assuming success")
	} else if exitCodeVal != nil {
		exitCode, ok := exitCodeVal.(int32)
		if ok && exitCode != 0 {
			return "", fmt.Errorf("command failed with exit code: %d", exitCode)
		}
	}

	// Get output using journalctl since systemd doesn't store command output directly
	output, err := getCommandOutput(unitName)
	if err != nil {
		return "", fmt.Errorf("failed to get command output: %w", err)
	}

	// Clean up the unit
	_, err = conn.StopUnit(unitName, "replace", nil)
	if err != nil {
		return "", fmt.Errorf("failed to stop unit: %w", err)
	}

	return output, nil
}

// getCommandOutput retrieves the command output from the journal for the given unit
func getCommandOutput(unitName string) (string, error) {
	// Use journalctl to get the command output
	cmd := exec.Command("journalctl", "-u", unitName, "--no-pager", "-o", "cat")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get unit output: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// RunScriptAsRoot executes a script content as root using systemd transient units
func RunScriptAsRoot(scriptContent string) (string, error) {
	// Execute the script using bash
	return RunCommandAsRoot("bash -c " + quoteCommand(scriptContent))
}

// quoteCommand ensures the command is properly quoted for shell execution
func quoteCommand(cmd string) string {
	return "'" + strings.Replace(cmd, "'", "'\\''", -1) + "'"
}
