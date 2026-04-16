package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
)

// Sender implements outbound.ChannelSender and gateway.TypingNotifier for Telegram.
type Sender struct {
	token       string
	httpClient  *http.Client
	baseURL     string
	maxAttempts int
	baseBackoff time.Duration
	sleep       func(time.Duration)
	now         func() time.Time

	mu                    sync.Mutex
	circuitBreakThreshold int
	circuitBreakCooldown  time.Duration
	consecutiveFailures   int
	breakerOpenUntil      time.Time
}

var ErrCircuitOpen = errors.New("telegram sender circuit breaker is open")

func NewSender(token string) *Sender {
	return &Sender{
		token:                 token,
		httpClient:            &http.Client{Timeout: 10 * time.Second},
		baseURL:               fmt.Sprintf("https://api.telegram.org/bot%s", token),
		maxAttempts:           3,
		baseBackoff:           200 * time.Millisecond,
		sleep:                 time.Sleep,
		now:                   time.Now,
		circuitBreakThreshold: 5,
		circuitBreakCooldown:  30 * time.Second,
	}
}

func NewSenderWithConfig(token string, timeout time.Duration, maxAttempts int, baseBackoff time.Duration) *Sender {
	sender := NewSender(token)
	if timeout > 0 {
		sender.httpClient.Timeout = timeout
	}
	if maxAttempts > 0 {
		sender.maxAttempts = maxAttempts
	}
	if baseBackoff > 0 {
		sender.baseBackoff = baseBackoff
	}
	return sender
}

func NewSenderFromConfig(cfg config.TelegramConfig) *Sender {
	sender := NewSenderWithConfig(cfg.BotToken, cfg.Timeout, cfg.MaxAttempts, cfg.BaseBackoff)
	if cfg.APIBaseURL != "" {
		sender.baseURL = strings.TrimRight(cfg.APIBaseURL, "/") + "/bot" + cfg.BotToken
	}
	if cfg.CircuitBreakThreshold > 0 {
		sender.circuitBreakThreshold = cfg.CircuitBreakThreshold
	}
	if cfg.CircuitBreakCooldown > 0 {
		sender.circuitBreakCooldown = cfg.CircuitBreakCooldown
	}
	return sender
}

type telegramAPIError struct {
	Method     string
	StatusCode int
	Body       string
}

func (e *telegramAPIError) Error() string {
	return fmt.Sprintf("telegram %s returned %d: %s", e.Method, e.StatusCode, e.Body)
}

type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type sendChatActionRequest struct {
	ChatID string `json:"chat_id"`
	Action string `json:"action"`
}

func (s *Sender) Send(ctx context.Context, _ string, msg domain.Message) error {
	chatID := msg.Metadata["chat_id"]
	if chatID == "" {
		return fmt.Errorf("telegram sender: no chat_id in message metadata")
	}

	return s.callAPI(ctx, "sendMessage", sendMessageRequest{
		ChatID: chatID,
		Text:   msg.Content,
	})
}

func (s *Sender) SendTyping(ctx context.Context, channelID string) error {
	chatID := channelID
	if strings.Contains(channelID, ":") {
		parts := strings.SplitN(channelID, ":", 2)
		chatID = parts[1]
	}

	return s.callAPI(ctx, "sendChatAction", sendChatActionRequest{
		ChatID: chatID,
		Action: "typing",
	})
}

func (s *Sender) callAPI(ctx context.Context, method string, payload any) error {
	if err := s.checkCircuit(method); err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", method, err)
	}

	var lastErr error
	attempts := s.maxAttempts
	if attempts <= 0 {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = s.callAPIAttempt(ctx, method, body)
		if lastErr == nil {
			s.recordSuccess()
			return nil
		}
		s.recordFailure(lastErr)
		if attempt == attempts || !isRetryableTelegramError(lastErr) {
			return lastErr
		}

		backoff := s.backoffForAttempt(attempt)
		select {
		case <-ctx.Done():
			return fmt.Errorf("call %s: %w", method, ctx.Err())
		default:
		}
		s.sleep(backoff)
	}

	return lastErr
}

func (s *Sender) checkCircuit(method string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.breakerOpenUntil.IsZero() || !s.now().Before(s.breakerOpenUntil) {
		if !s.breakerOpenUntil.IsZero() && !s.now().Before(s.breakerOpenUntil) {
			s.breakerOpenUntil = time.Time{}
		}
		return nil
	}

	return fmt.Errorf("call %s: %w until %s", method, ErrCircuitOpen, s.breakerOpenUntil.UTC().Format(time.RFC3339))
}

func (s *Sender) recordSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.consecutiveFailures = 0
	s.breakerOpenUntil = time.Time{}
}

func (s *Sender) recordFailure(err error) {
	if !isRetryableTelegramError(err) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.consecutiveFailures++
	if s.consecutiveFailures < s.circuitBreakThreshold {
		return
	}
	s.breakerOpenUntil = s.now().Add(s.circuitBreakCooldown)
	s.consecutiveFailures = 0
}

func (s *Sender) callAPIAttempt(ctx context.Context, method string, body []byte) error {
	url := fmt.Sprintf("%s/%s", s.baseURL, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &telegramAPIError{
			Method:     method,
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	return nil
}

func (s *Sender) backoffForAttempt(attempt int) time.Duration {
	if attempt <= 0 {
		return s.baseBackoff
	}
	multiplier := math.Pow(2, float64(attempt-1))
	return time.Duration(float64(s.baseBackoff) * multiplier)
}

func isRetryableTelegramError(err error) bool {
	if apiErr, ok := errors.AsType[*telegramAPIError](err); ok {
		return apiErr.StatusCode >= http.StatusInternalServerError
	}
	return true
}
