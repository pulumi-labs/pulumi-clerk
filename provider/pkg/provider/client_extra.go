// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// doMultipart uploads content as the single multipart form file field. The
// Platform API takes application logos and favicons this way. out may be nil.
func (c *client) doMultipart(ctx context.Context, method, path, field string, content []byte, out any) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, "upload")
	if err != nil {
		return fmt.Errorf("clerk: build upload: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("clerk: build upload: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("clerk: build upload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &buf)
	if err != nil {
		return fmt.Errorf("clerk: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", w.FormDataContentType())
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
