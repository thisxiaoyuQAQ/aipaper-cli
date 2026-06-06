package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultUserAgent = "aipaper-cli/0.1"

type httpProvider struct {
	name    string
	baseURL string
	client  *http.Client
}

func (p httpProvider) Name() string {
	return p.name
}

func (p httpProvider) getJSON(ctx context.Context, endpoint string, values url.Values, target any) error {
	reqURL, err := joinURL(p.baseURL, endpoint, values)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ProviderError{Code: CodeSearchTimeout, Source: p.name, Message: ctx.Err().Error(), Retryable: true}
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return ProviderError{Code: CodeSearchRateLimited, Source: p.name, Message: "rate limited", Retryable: true}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return ProviderError{
			Code:      "REFERENCE_SEARCH_FAILED",
			Source:    p.name,
			Message:   fmt.Sprintf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
			Retryable: resp.StatusCode >= 500,
		}
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(target); err != nil {
		return err
	}
	return nil
}

func (p httpProvider) getText(ctx context.Context, endpoint string, values url.Values) ([]byte, error) {
	reqURL, err := joinURL(p.baseURL, endpoint, values)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ProviderError{Code: CodeSearchTimeout, Source: p.name, Message: ctx.Err().Error(), Retryable: true}
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ProviderError{Code: CodeSearchRateLimited, Source: p.name, Message: "rate limited", Retryable: true}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, ProviderError{
			Code:      "REFERENCE_SEARCH_FAILED",
			Source:    p.name,
			Message:   fmt.Sprintf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
			Retryable: resp.StatusCode >= 500,
		}
	}
	return io.ReadAll(resp.Body)
}

func joinURL(base, endpoint string, values url.Values) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(base, "/") + "/" + strings.TrimLeft(endpoint, "/"))
	if err != nil {
		return "", err
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}
