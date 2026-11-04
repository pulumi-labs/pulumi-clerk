// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// defaultAPIURL is Clerk's production Backend and Platform API endpoint.
const defaultAPIURL = "https://api.clerk.com/v1"

// apiError is a non-2xx answer from the Clerk API.
type apiError struct {
	Status  int
	Code    string
	Message string
	Body    string
}

func (e *apiError) Error() string {
	if e.Message != "" {
		if e.Code != "" {
			return fmt.Sprintf("clerk API error (HTTP %d, %s): %s", e.Status, e.Code, e.Message)
		}
		return fmt.Sprintf("clerk API error (HTTP %d): %s", e.Status, e.Message)
	}
	return fmt.Sprintf("clerk API error (HTTP %d): %s", e.Status, e.Body)
}

// isNotFound reports whether err is Clerk saying the object does not exist.
func isNotFound(err error) bool {
	var ae *apiError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

// client is a minimal JSON client for the Clerk Backend API and the Platform
// API (beta); both share a base URL and differ only in the bearer key.
type client struct {
	http    *http.Client
	baseURL string
	key     string
}

func newClient(baseURL, key string) *client {
	if baseURL == "" {
		baseURL = defaultAPIURL
	}
	return &client{
		http:    &http.Client{Timeout: 60 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
	}
}

// do sends one JSON request. body and out may be nil.
func (c *client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("clerk: encode request: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return fmt.Errorf("clerk: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("clerk: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("clerk: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ae := &apiError{Status: resp.StatusCode, Body: strings.TrimSpace(string(data))}
		var env struct {
			Errors []struct {
				Code        string `json:"code"`
				Message     string `json:"message"`
				LongMessage string `json:"long_message"`
			} `json:"errors"`
		}
		if json.Unmarshal(data, &env) == nil && len(env.Errors) > 0 {
			ae.Code = env.Errors[0].Code
			ae.Message = env.Errors[0].LongMessage
			if ae.Message == "" {
				ae.Message = env.Errors[0].Message
			}
		}
		return ae
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("clerk: decode %s %s response: %w", method, path, err)
		}
	}
	return nil
}

// listAll reads every page of a list endpoint. Clerk answers either a bare
// JSON array or {"data": [...], "total_count": n}; both are accepted.
func listAll[T any](ctx context.Context, c *client, path string, query url.Values) ([]T, error) {
	const pageSize = 100
	var all []T
	for offset := 0; ; offset += pageSize {
		q := url.Values{}
		for k, v := range query {
			q[k] = v
		}
		q.Set("limit", strconv.Itoa(pageSize))
		q.Set("offset", strconv.Itoa(offset))
		var raw json.RawMessage
		if err := c.do(ctx, http.MethodGet, path+"?"+q.Encode(), nil, &raw); err != nil {
			return nil, err
		}
		var page []T
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) > 0 && trimmed[0] == '[' {
			if err := json.Unmarshal(trimmed, &page); err != nil {
				return nil, fmt.Errorf("clerk: decode list %s: %w", path, err)
			}
		} else {
			var env struct {
				Data []T `json:"data"`
			}
			if err := json.Unmarshal(trimmed, &env); err != nil {
				return nil, fmt.Errorf("clerk: decode list %s: %w", path, err)
			}
			page = env.Data
		}
		all = append(all, page...)
		if len(page) < pageSize {
			return all, nil
		}
	}
}

// millisToRFC3339 renders a Clerk millisecond timestamp.
func millisToRFC3339(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 120*time.Second)
}
