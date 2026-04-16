package confirm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/enolalab/alfred/internal/domain"
)

type CLIConfirm struct {
	in  io.Reader
	out io.Writer
}

func NewCLIConfirm(in io.Reader, out io.Writer) *CLIConfirm {
	return &CLIConfirm{in: in, out: out}
}

func (c *CLIConfirm) Confirm(_ context.Context, call domain.ToolCall) (bool, error) {
	fmt.Fprintf(c.out, "\n[SECURITY] Tool %q wants to execute:\n", call.ToolName)
	fmt.Fprintf(c.out, "  Parameters: %s\n", string(call.Parameters))
	fmt.Fprint(c.out, "  Allow? [y/N]: ")

	scanner := bufio.NewScanner(c.in)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}
