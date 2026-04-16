package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
	"github.com/enolalab/alfred/internal/port/inbound"
)

type REPL struct {
	handler        inbound.ChatHandler
	conversationID string
	in             io.Reader
	out            io.Writer
}

func NewREPL(handler inbound.ChatHandler, conversationID string, in io.Reader, out io.Writer) *REPL {
	return &REPL{
		handler:        handler,
		conversationID: conversationID,
		in:             in,
		out:            out,
	}
}

func (r *REPL) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(r.in)
	fmt.Fprintln(r.out, "Alfred — Autonomous AI Agent")
	fmt.Fprintln(r.out, "Type your message (Ctrl+D to exit)")
	fmt.Fprintln(r.out)

	for {
		fmt.Fprint(r.out, "you> ")
		if !scanner.Scan() {
			break
		}

		input := scanner.Text()
		if input == "" {
			continue
		}

		msg := domain.Message{
			ID:             fmt.Sprintf("msg_%d", time.Now().UnixNano()),
			ConversationID: r.conversationID,
			Role:           vo.RoleUser,
			Content:        input,
			Platform:       vo.PlatformCLI,
			CreatedAt:      time.Now(),
		}

		resp, err := r.handler.HandleMessage(ctx, msg)
		if err != nil {
			fmt.Fprintf(r.out, "error: %v\n\n", err)
			continue
		}

		fmt.Fprintf(r.out, "\nalfred> %s\n\n", resp.Content)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	fmt.Fprintln(r.out, "\nGoodbye!")
	return nil
}
