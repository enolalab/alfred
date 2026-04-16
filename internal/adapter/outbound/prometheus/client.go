package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
)

type Client struct {
	baseURL        *url.URL
	httpClient     *http.Client
	bearerToken    string
	defaultCluster string
	mode           string
	maxSeries      int
	maxSamples     int
	now            func() time.Time

	mu                    sync.Mutex
	circuitBreakThreshold int
	circuitBreakCooldown  time.Duration
	consecutiveFailures   int
	breakerOpenUntil      time.Time
}

var ErrCircuitOpen = errors.New("prometheus client circuit breaker is open")

func NewClient(cfg config.PrometheusToolConfig) (*Client, error) {
	mode, err := normalizeMode(cfg.Mode)
	if err != nil {
		return nil, err
	}
	baseURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse prometheus base_url: %w", err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("prometheus base_url must be absolute, got %q", cfg.BaseURL)
	}
	return &Client{
		baseURL:               baseURL,
		httpClient:            &http.Client{Timeout: cfg.Timeout},
		bearerToken:           cfg.BearerToken,
		defaultCluster:        cfg.DefaultCluster,
		mode:                  mode,
		maxSeries:             cfg.MaxSeries,
		maxSamples:            cfg.MaxSamples,
		now:                   time.Now,
		circuitBreakThreshold: cfg.CircuitBreakThreshold,
		circuitBreakCooldown:  cfg.CircuitBreakCooldown,
	}, nil
}

func (c *Client) QueryRange(ctx context.Context, cluster, query string, start, end time.Time, step time.Duration) (*domain.PrometheusQueryResult, error) {
	if err := c.checkCircuit(); err != nil {
		return nil, err
	}
	if cluster != "" && c.defaultCluster != "" && cluster != c.defaultCluster {
		return nil, fmt.Errorf("cluster %q is not allowed; expected %q", cluster, c.defaultCluster)
	}

	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, "/api/v1/query_range")
	values := endpoint.Query()
	values.Set("query", query)
	values.Set("start", start.UTC().Format(time.RFC3339))
	values.Set("end", end.UTC().Format(time.RFC3339))
	values.Set("step", fmt.Sprintf("%.0f", step.Seconds()))
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build prometheus request: %w", err)
	}
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		wrapped := fmt.Errorf("query prometheus: %w", err)
		c.recordFailure(wrapped)
		return nil, wrapped
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("prometheus returned status %d", resp.StatusCode)
		c.recordFailure(err)
		return nil, err
	}

	var payload struct {
		Status   string   `json:"status"`
		Warnings []string `json:"warnings"`
		Data     struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Values [][]any           `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		wrapped := fmt.Errorf("decode prometheus response: %w", err)
		c.recordFailure(wrapped)
		return nil, wrapped
	}
	if payload.Status != "success" {
		err := fmt.Errorf("prometheus status %q", payload.Status)
		c.recordFailure(err)
		return nil, err
	}

	result := &domain.PrometheusQueryResult{
		Query:      query,
		ExecutedAt: time.Now().UTC().Format(time.RFC3339),
		ResultType: payload.Data.ResultType,
		Warnings:   payload.Warnings,
		Series:     make([]domain.PrometheusSeries, 0, len(payload.Data.Result)),
	}
	for i, series := range payload.Data.Result {
		if i >= c.maxSeries {
			result.Truncated = true
			break
		}
		item := domain.PrometheusSeries{
			Metric: series.Metric,
			Values: make([]domain.PrometheusSample, 0, len(series.Values)),
		}
		for j, sample := range series.Values {
			if j >= c.maxSamples {
				result.Truncated = true
				break
			}
			if len(sample) != 2 {
				continue
			}
			item.Values = append(item.Values, domain.PrometheusSample{
				Timestamp: fmt.Sprint(sample[0]),
				Value:     fmt.Sprint(sample[1]),
			})
		}
		result.Series = append(result.Series, item)
	}
	c.recordSuccess()
	return result, nil
}

func (c *Client) checkCircuit() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if c.breakerOpenUntil.IsZero() || !now.Before(c.breakerOpenUntil) {
		if !c.breakerOpenUntil.IsZero() && !now.Before(c.breakerOpenUntil) {
			c.breakerOpenUntil = time.Time{}
		}
		return nil
	}

	return fmt.Errorf("%w until %s", ErrCircuitOpen, c.breakerOpenUntil.UTC().Format(time.RFC3339))
}

func (c *Client) recordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFailures = 0
	c.breakerOpenUntil = time.Time{}
}

func (c *Client) recordFailure(err error) {
	if !isRetryablePrometheusError(err) {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFailures++
	if c.consecutiveFailures < c.circuitBreakThreshold {
		return
	}
	c.breakerOpenUntil = c.now().Add(c.circuitBreakCooldown)
	c.consecutiveFailures = 0
}

func isRetryablePrometheusError(err error) bool {
	if errors.Is(err, ErrCircuitOpen) {
		return false
	}
	var statusCode int
	if _, scanErr := fmt.Sscanf(err.Error(), "prometheus returned status %d", &statusCode); scanErr == nil {
		return statusCode >= http.StatusInternalServerError
	}
	return true
}

func normalizeMode(mode string) (string, error) {
	switch mode {
	case "", "auto":
		return "auto", nil
	case "in_cluster", "ex_cluster":
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported prometheus mode %q", mode)
	}
}
