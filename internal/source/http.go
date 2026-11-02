package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultAttempts   = 4
	defaultRetryDelay = 400 * time.Millisecond
	maxResponseBytes  = 32 << 20
)

// Client is the shared HTTP client with retry and backoff behavior.
type Client struct {
	HTTP     *http.Client
	Attempts int
	Delay    time.Duration
}

// NewClient returns the production HTTP client.
func NewClient() *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: defaultTimeout},
		Attempts: defaultAttempts,
		Delay:    defaultRetryDelay,
	}
}

// Get performs a GET and retries 429 and 5xx responses with exponential backoff.
func (c *Client) Get(ctx context.Context, target string) (*http.Response, error) {
	attempts := c.Attempts
	if attempts < 1 {
		attempts = 1
	}
	delay := c.Delay
	if delay <= 0 {
		delay = defaultRetryDelay
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		setProviderHeaders(request)
		response, err := c.client().Do(request)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		} else if response.StatusCode >= 200 && response.StatusCode < 300 {
			return response, nil
		} else {
			lastErr = fmt.Errorf("HTTP %s", response.Status)
			retryable := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
			wait := retryAfter(response, delay)
			drainAndClose(response)
			if !retryable {
				return nil, lastErr
			}
			delay = wait
		}
		if attempt < attempts {
			wait := delay + jitter(delay/4)
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			delay *= 2
		}
	}
	if lastErr == nil {
		lastErr = errors.New("request failed")
	}
	return nil, lastErr
}

// GetJSON decodes a JSON response body into target.
func (c *Client) GetJSON(ctx context.Context, target string, value any) error {
	response, err := c.Get(ctx, target)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(value); err != nil {
		return fmt.Errorf("decode %s: %w", target, err)
	}
	return nil
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: defaultTimeout}
}

func retryAfter(response *http.Response, fallback time.Duration) time.Duration {
	value := response.Header.Get("Retry-After")
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}
	return fallback
}

func jitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(max)))
}

func drainAndClose(response *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	_ = response.Body.Close()
}
