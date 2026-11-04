package quran

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	requestTimeout   = 30 * time.Second
	maxAttempts      = 4
	retryBaseDelay   = 400 * time.Millisecond
	maxResponseBytes = 32 << 20
	maxErrorBytes    = 512
)

type apiError struct {
	URL        string
	StatusCode int
	Status     string
	Body       string
}

func (e *apiError) Error() string {
	message := fmt.Sprintf("quran.com API: %s returned HTTP %s", e.URL, e.Status)
	if e.Body != "" {
		message += ": " + e.Body
	}
	return message
}

// get performs one GET with retries for 429 and 5xx responses.
func (c *Client) get(ctx context.Context, rawURL string) (*http.Response, error) {
	attempts := c.attempts
	if attempts < 1 {
		attempts = 1
	}
	delay := c.delay
	if delay <= 0 {
		delay = retryBaseDelay
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("quran.com API: build request: %w", err)
		}
		setHeaders(request)
		response, err := c.http.Do(request)
		if err != nil {
			lastErr = fmt.Errorf("quran.com API: request %s failed: %w", rawURL, err)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		} else if response.StatusCode >= 200 && response.StatusCode < 300 {
			return response, nil
		} else {
			body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBytes))
			lastErr = &apiError{
				URL:        rawURL,
				StatusCode: response.StatusCode,
				Status:     response.Status,
				Body:       strings.TrimSpace(string(body)),
			}
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
		lastErr = errors.New("quran.com API: request failed")
	}
	return nil, lastErr
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, value any) error {
	rawURL := c.baseURL + path
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}
	response, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(value); err != nil {
		return fmt.Errorf("quran.com API: decode %s: %w", rawURL, err)
	}
	return nil
}

// Download writes a remote file to destination using the shared HTTP client.
func (c *Client) Download(ctx context.Context, rawURL, destination string) error {
	response, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(destination)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(destination)
		return closeErr
	}
	return nil
}

func setHeaders(request *http.Request) {
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(os.Getenv(QuranFoundationTokenEnv)); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
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
