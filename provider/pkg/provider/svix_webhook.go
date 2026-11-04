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

// svixWebhookID is the synthetic ID of the per-instance Svix integration.
const svixWebhookID = "svix_webhook"

// SvixWebhook enables the Svix webhook integration of a Clerk instance. It is
// a singleton: an instance has at most one Svix app.
type SvixWebhook struct{}

// SvixWebhookArgs are the inputs of a SvixWebhook; there are none.
type SvixWebhookArgs struct{}

// SvixWebhookState is the stored state of a SvixWebhook.
type SvixWebhookState struct {
	SvixWebhookArgs
	SvixURL string `pulumi:"svixUrl"`
}

func (r *SvixWebhook) Annotate(a infer.Annotator) {
	a.Describe(&r, "The Svix webhook integration of a Clerk instance. Creating it provisions the Svix app that delivers Clerk webhooks; deleting it removes the app. An instance has at most one, so the ID is always svix_webhook. Import with the ID svix_webhook.")
}

func (s *SvixWebhookState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.SvixURL, "A fresh, short-lived URL of the Svix dashboard for managing webhook endpoints. Refreshing the resource generates a new one.")
}

var _ infer.CustomResource[SvixWebhookArgs, SvixWebhookState] = (*SvixWebhook)(nil)
var _ infer.CustomUpdate[SvixWebhookArgs, SvixWebhookState] = (*SvixWebhook)(nil)
var _ infer.CustomRead[SvixWebhookArgs, SvixWebhookState] = (*SvixWebhook)(nil)
var _ infer.CustomDelete[SvixWebhookState] = (*SvixWebhook)(nil)

type svixWebhookJSON struct {
	SvixURL string `json:"svix_url"`
}

func (r *SvixWebhook) Create(ctx context.Context, req infer.CreateRequest[SvixWebhookArgs]) (infer.CreateResponse[SvixWebhookState], error) {
	if req.DryRun {
		return infer.CreateResponse[SvixWebhookState]{}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[SvixWebhookState]{}, err
	}
	var out svixWebhookJSON
	if err := c.do(ctx, http.MethodPost, "/webhooks/svix", nil, &out); err != nil {
		return infer.CreateResponse[SvixWebhookState]{}, fmt.Errorf("clerk: create Svix webhook integration: %w", err)
	}
	return infer.CreateResponse[SvixWebhookState]{ID: svixWebhookID, Output: SvixWebhookState{SvixURL: out.SvixURL}}, nil
}

// Update has nothing to change: the resource has no inputs.
func (r *SvixWebhook) Update(_ context.Context, req infer.UpdateRequest[SvixWebhookArgs, SvixWebhookState]) (infer.UpdateResponse[SvixWebhookState], error) {
	return infer.UpdateResponse[SvixWebhookState]{Output: req.State}, nil
}

// refreshSvixURL asks Clerk for a new dashboard URL. The API has no get for the
// Svix app, but this call only succeeds when the app exists.
func refreshSvixURL(ctx context.Context, c *client) (*svixWebhookJSON, error) {
	var out svixWebhookJSON
	if err := c.do(ctx, http.MethodPost, "/webhooks/svix_url", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *SvixWebhook) Read(ctx context.Context, req infer.ReadRequest[SvixWebhookArgs, SvixWebhookState]) (infer.ReadResponse[SvixWebhookArgs, SvixWebhookState], error) {
	empty := infer.ReadResponse[SvixWebhookArgs, SvixWebhookState]{}
	if req.ID != svixWebhookID {
		return empty, fmt.Errorf("clerk: the Svix webhook integration has the fixed ID %q, got %q", svixWebhookID, req.ID)
	}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	out, err := refreshSvixURL(ctx, c)
	if err != nil {
		if isNotFound(err) {
			return empty, nil
		}
		return empty, fmt.Errorf("clerk: read Svix webhook integration: %w", err)
	}
	return infer.ReadResponse[SvixWebhookArgs, SvixWebhookState]{ID: svixWebhookID, State: SvixWebhookState{SvixURL: out.SvixURL}}, nil
}

func (r *SvixWebhook) Delete(ctx context.Context, req infer.DeleteRequest[SvixWebhookState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/webhooks/svix", nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete Svix webhook integration: %w", err)
	}
	return infer.DeleteResponse{}, nil
}

// GetSvixWebhook reports the Svix integration of the instance.
type GetSvixWebhook struct{}

// GetSvixWebhookArgs has no fields.
type GetSvixWebhookArgs struct{}

// GetSvixWebhookResult is the Svix integration.
type GetSvixWebhookResult struct {
	ID      string `pulumi:"svixWebhookId"`
	SvixURL string `pulumi:"svixUrl"`
}

func (f *GetSvixWebhook) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up the Svix webhook integration of the Clerk instance and get a fresh Svix dashboard URL. Fails when the integration has not been created.")
}

func (f *GetSvixWebhook) Invoke(ctx context.Context, _ infer.FunctionRequest[GetSvixWebhookArgs]) (infer.FunctionResponse[GetSvixWebhookResult], error) {
	out := infer.FunctionResponse[GetSvixWebhookResult]{}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	got, err := refreshSvixURL(ctx, c)
	if err != nil {
		return out, lookupErr("Svix webhook integration", "for this instance", err)
	}
	out.Output = GetSvixWebhookResult{ID: svixWebhookID, SvixURL: got.SvixURL}
	return out, nil
}
