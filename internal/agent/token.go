package agent

import (
	"fmt"
	"os"
	"strings"
)

func LoadToken(path string) (string, error) {
	if path == "" {
		return strings.TrimSpace(os.Getenv("SERVTERM_AGENT_TOKEN")), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return "", fmt.Errorf("token file %s is empty", path)
	}
	return token, nil
}
