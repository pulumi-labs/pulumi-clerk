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
	"context"
	"fmt"
	"net/http"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// RedirectUrl allowlists a URL for redirect-based authentication flows.
type RedirectUrl struct{}

// RedirectUrlArgs are the inputs of a RedirectUrl.
type RedirectUrlArgs struct {
	// Url is the URL to allow as a redirect target.
	Url string `pulumi:"url" provider:"replaceOnChanges"`
}

// RedirectUrlState is the stored state of a RedirectUrl.
type RedirectUrlState struct {
	RedirectUrlArgs
	CreatedAt string `pulumi:"createdAt"`
	UpdatedAt string `pulumi:"updatedAt"`
}

func (r *RedirectUrl) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk redirect URL. Redirect URLs allowlist the URLs that OAuth and other redirect-based authentication flows may send users back to. Redirect URLs cannot be updated: changing the URL replaces the entry.")
}

func (a *RedirectUrlArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Url, "The URL to allow for redirect-based authentication flows. Changing this forces a new resource.")
}

func (s *RedirectUrlState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.CreatedAt, "When the redirect URL was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the redirect URL was last updated (RFC 3339).")
}

var _ infer.CustomResource[RedirectUrlArgs, RedirectUrlState] = (*RedirectUrl)(nil)
var _ infer.CustomRead[RedirectUrlArgs, RedirectUrlState] = (*RedirectUrl)(nil)
var _ infer.CustomDelete[RedirectUrlState] = (*RedirectUrl)(nil)

// redirectUrlJSON is the API shape.
type redirectUrlJSON struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (j redirectUrlJSON) state() RedirectUrlState {
	return RedirectUrlState{
		RedirectUrlArgs: RedirectUrlArgs{Url: j.URL},
		CreatedAt:       millisToRFC3339(j.CreatedAt),
		UpdatedAt:       millisToRFC3339(j.UpdatedAt),
	}
}

// getRedirectUrl fetches a redirect URL by ID; nil means it does not exist.
func getRedirectUrl(ctx context.Context, c *client, id string) (*redirectUrlJSON, error) {
	var out redirectUrlJSON
	if err := c.do(ctx, http.MethodGet, "/redirect_urls/"+id, nil, &out); err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// findRedirectUrlByURL looks a redirect URL up by its URL through the list.
func findRedirectUrlByURL(ctx context.Context, c *client, url string) (*redirectUrlJSON, error) {
	items, err := listAll[redirectUrlJSON](ctx, c, "/redirect_urls", nil)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].URL == url {
			return &items[i], nil
		}
	}
	return nil, nil
}

func (r *RedirectUrl) Create(ctx context.Context, req infer.CreateRequest[RedirectUrlArgs]) (infer.CreateResponse[RedirectUrlState], error) {
	if req.DryRun {
		return infer.CreateResponse[RedirectUrlState]{Output: RedirectUrlState{RedirectUrlArgs: req.Inputs}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[RedirectUrlState]{}, err
	}
	var out redirectUrlJSON
	if err := c.do(ctx, http.MethodPost, "/redirect_urls", map[string]any{"url": req.Inputs.Url}, &out); err != nil {
		return infer.CreateResponse[RedirectUrlState]{}, fmt.Errorf("clerk: create redirect URL %q: %w", req.Inputs.Url, err)
	}
	return infer.CreateResponse[RedirectUrlState]{ID: out.ID, Output: out.state()}, nil
}

func (r *RedirectUrl) Read(ctx context.Context, req infer.ReadRequest[RedirectUrlArgs, RedirectUrlState]) (infer.ReadResponse[RedirectUrlArgs, RedirectUrlState], error) {
	empty := infer.ReadResponse[RedirectUrlArgs, RedirectUrlState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := getRedirectUrl(ctx, c, req.ID)
	if err != nil {
		return empty, fmt.Errorf("clerk: read redirect URL %q: %w", req.ID, err)
	}
	if got == nil {
		return empty, nil
	}
	st := got.state()
	return infer.ReadResponse[RedirectUrlArgs, RedirectUrlState]{ID: got.ID, Inputs: st.RedirectUrlArgs, State: st}, nil
}

func (r *RedirectUrl) Delete(ctx context.Context, req infer.DeleteRequest[RedirectUrlState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/redirect_urls/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete redirect URL %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetRedirectUrl looks up a redirect URL by ID or URL.
type GetRedirectUrl struct{}

// GetRedirectUrlArgs selects the redirect URL; set exactly one of the fields.
type GetRedirectUrlArgs struct {
	ID  *string `pulumi:"redirectUrlId,optional"`
	Url *string `pulumi:"url,optional"`
}

// GetRedirectUrlResult is the looked-up redirect URL.
type GetRedirectUrlResult struct {
	ID        string `pulumi:"redirectUrlId"`
	Url       string `pulumi:"url"`
	CreatedAt string `pulumi:"createdAt"`
	UpdatedAt string `pulumi:"updatedAt"`
}

func (f *GetRedirectUrl) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk redirect URL by ID or by URL.")
}

func (a *GetRedirectUrlArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The redirect URL ID. Set either redirectUrlId or url.")
	ann.Describe(&a.Url, "The allowlisted redirect URL. Set either redirectUrlId or url.")
}

func (f *GetRedirectUrl) Invoke(ctx context.Context, req infer.FunctionRequest[GetRedirectUrlArgs]) (infer.FunctionResponse[GetRedirectUrlResult], error) {
	out := infer.FunctionResponse[GetRedirectUrlResult]{}
	id, u := derefStr(req.Input.ID), derefStr(req.Input.Url)
	if (id == "") == (u == "") {
		return out, fmt.Errorf("clerk: getRedirectUrl needs exactly one of redirectUrlId or url")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	var got *redirectUrlJSON
	if id != "" {
		got, err = getRedirectUrl(ctx, c, id)
	} else {
		got, err = findRedirectUrlByURL(ctx, c, u)
	}
	if err != nil {
		return out, lookupErr("redirect URL", id+u, err)
	}
	if got == nil {
		return out, lookupErr("redirect URL", fmt.Sprintf("%q", id+u), &apiError{Status: http.StatusNotFound})
	}
	st := got.state()
	out.Output = GetRedirectUrlResult{ID: got.ID, Url: got.URL, CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt}
	return out, nil
}
