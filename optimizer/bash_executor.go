package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// BashExecutor is responsible for executing bash scripts
type BashExecutor struct {
	// Default environment variables to be passed to all scripts
	defaultEnvVars map[string]string
	// Whether to stream output to console by default
	streamOutput bool
}

// NewBashExecutor creates a new instance of BashExecutor
func NewBashExecutor() *BashExecutor {
	return &BashExecutor{
		defaultEnvVars: make(map[string]string),
		streamOutput:   false, // Default to not streaming for backward compatibility
	}
}

// SetStreamOutput sets whether to stream output to console by default
func (e *BashExecutor) SetStreamOutput(stream bool) {
	e.streamOutput = stream
}

// SetDefaultEnvVar sets a default environment variable that will be passed to all scripts
func (e *BashExecutor) SetDefaultEnvVar(key, value string) {
	e.defaultEnvVars[key] = value
}

// ExecuteScriptStreaming runs a bash script at the given path with the provided parameters
// and streams the stdout and stderr to the console in real-time
func (e *BashExecutor) ExecuteScriptStreaming(scriptPath string, params []string, envVars map[string]string) error {
	// Verify the script exists
	scriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("script not found at path: %s", scriptPath)
	}

	// Create the command
	cmd := exec.Command("bash", append([]string{scriptPath}, params...)...)

	// Set up environment variables
	env := os.Environ() // Start with current environment

	// Add default environment variables
	for key, value := range e.defaultEnvVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Add script-specific environment variables (these override defaults)
	for key, value := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd.Env = env

	// Set up stdout and stderr to stream directly to the console
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("script execution failed: %v", err)
	}

	return nil
}
