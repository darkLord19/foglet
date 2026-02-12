package process

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Manager handles process management for dev servers
type Manager struct {
	// Can be expanded to track PIDs, logs, etc.
}

// New creates a new process manager
func New() *Manager {
	return &Manager{}
}

// GetPortInfo returns information about what's using a port
func (m *Manager) GetPortInfo(port int) (*PortInfo, error) {
	// Use lsof to check port usage
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-t")
	output, err := cmd.Output()
	if err != nil {
		// Port not in use
		return nil, nil
	}

	pidStr := strings.TrimSpace(string(output))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, err
	}

	return &PortInfo{
		Port: port,
		PID:  pid,
	}, nil
}

// IsPortInUse checks if a port is currently in use
func (m *Manager) IsPortInUse(port int) bool {
	info, _ := m.GetPortInfo(port)
	return info != nil
}

// FindAvailablePort finds the next available port starting from the given port
func (m *Manager) FindAvailablePort(startPort int) int {
	for port := startPort; port < startPort+100; port++ {
		if !m.IsPortInUse(port) {
			return port
		}
	}
	return -1
}

// KillPort kills the process using a port
func (m *Manager) KillPort(port int) error {
	info, err := m.GetPortInfo(port)
	if err != nil || info == nil {
		return fmt.Errorf("port %d not in use", port)
	}

	cmd := exec.Command("kill", strconv.Itoa(info.PID))
	return cmd.Run()
}

// Start starts a dev server command
func (m *Manager) Start(workdir, command string) (*Process, error) {
	// Parse command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = workdir

	// Setup stdout/stderr capture
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	proc := &Process{
		PID:     cmd.Process.Pid,
		Command: command,
		cmd:     cmd,
	}

	// Start goroutines to read output
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			// Could log this to a file
			fmt.Println(scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// Could log this to a file
			fmt.Fprintln(os.Stderr, scanner.Text())
		}
	}()

	return proc, nil
}

// PortInfo contains information about a port
type PortInfo struct {
	Port int
	PID  int
}

// Process represents a running process
type Process struct {
	PID     int
	Command string
	cmd     *exec.Cmd
}

// Wait waits for the process to finish
func (p *Process) Wait() error {
	return p.cmd.Wait()
}

// Kill terminates the process
func (p *Process) Kill() error {
	return p.cmd.Process.Kill()
}
