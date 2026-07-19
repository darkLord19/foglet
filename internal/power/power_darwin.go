//go:build darwin

package power

import (
	"log"
	"os/exec"
)

func (ih *Inhibitor) startPlatform() {
	cmd := exec.Command("caffeinate", "-i", "-s")
	if err := cmd.Start(); err != nil {
		log.Printf("power: failed to start caffeinate: %v", err)
		return
	}
	ih.cmd = cmd
	log.Printf("power: keep-awake acquired (pid %d)", cmd.Process.Pid)
}

func (ih *Inhibitor) stopPlatform() {
	if ih.cmd != nil && ih.cmd.Process != nil {
		if err := ih.cmd.Process.Kill(); err != nil {
			log.Printf("power: failed to kill caffeinate: %v", err)
		} else {
			log.Printf("power: keep-awake released")
		}
		_ = ih.cmd.Wait() // reap child process
		ih.cmd = nil
	}
}
