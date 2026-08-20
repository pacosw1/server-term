package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
	"github.com/franciscosainzwilliams/server-term/internal/config"
)

type sshPane struct {
	file     *os.File
	cmd      *exec.Cmd
	output   chan sshOutputMsg
	emulator *vt.Emulator
	mu       sync.Mutex
	closed   bool
}
type sshOutputMsg struct {
	Data string
	Err  error
}

func startSSHPane(server config.Server) (*sshPane, error) {
	args := []string{"-tt"}
	if server.Port != 0 {
		args = append(args, "-p", strconv.Itoa(server.Port))
	}
	if server.IdentityFile != "" {
		args = append(args, "-i", config.ExpandHome(server.IdentityFile))
	}
	target := server.Address
	if server.User != "" {
		target = server.User + "@" + target
	}
	args = append(args, target)
	cmd := exec.Command("ssh", args...)
	file, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	s := &sshPane{file: file, cmd: cmd, output: make(chan sshOutputMsg, 16), emulator: vt.NewEmulator(120, 40)}
	go func() {
		buf := make([]byte, 8192)
		for {
			n, e := file.Read(buf)
			if n > 0 {
				s.output <- sshOutputMsg{Data: string(buf[:n])}
			}
			if e != nil {
				s.output <- sshOutputMsg{Err: e}
				close(s.output)
				return
			}
		}
	}()
	return s, nil
}
func (s *sshPane) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	_ = s.file.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}
func (s *sshPane) write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("ssh session closed")
	}
	_, err := s.file.Write(data)
	return err
}

func (s *sshPane) resize(cols, rows int) {
	_ = pty.Setsize(s.file, &pty.Winsize{Cols: uint16(max(1, cols)), Rows: uint16(max(1, rows))})
	s.emulator.Resize(max(1, cols), max(1, rows))
}
func (s *sshPane) feed(data string) string {
	_, _ = s.emulator.WriteString(data)
	return s.emulator.Render()
}
func (s *sshPane) read() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-s.output
		if !ok {
			return sshOutputMsg{Err: io.EOF}
		}
		return msg
	}
}
func keyBytes(msg tea.KeyMsg) []byte {
	switch msg.String() {
	case "enter":
		return []byte{'\r'}
	case "backspace":
		return []byte{0x7f}
	case "tab":
		return []byte{'\t'}
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	case "delete":
		return []byte("\x1b[3~")
	}
	if len(msg.Runes) > 0 {
		return []byte(string(msg.Runes))
	}
	return []byte(msg.String())
}
