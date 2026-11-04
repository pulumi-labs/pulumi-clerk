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

// AllowlistIdentifier allows an email address, phone number or web3 wallet
// (or an email wildcard such as *@example.com) to sign up to a Clerk instance
// that restricts sign-ups to its allowlist.
type AllowlistIdentifier struct{}

// AllowlistIdentifierArgs are the inputs of an AllowlistIdentifier.
type AllowlistIdentifierArgs struct {
	// Identifier is the email address, phone number or web3 wallet to allow. Wildcards such as *@example.com are supported for email addresses.
	Identifier string `pulumi:"identifier" provider:"replaceOnChanges"`
	// Notify sends an invitation email to the identifier when it is created.
	Notify *bool `pulumi:"notify,optional" provider:"replaceOnChanges"`
}

// AllowlistIdentifierState is the stored state of an AllowlistIdentifier.
type AllowlistIdentifierState struct {
	AllowlistIdentifierArgs
	IdentifierType string `pulumi:"identifierType"`
	CreatedAt      string `pulumi:"createdAt"`
	UpdatedAt      string `pulumi:"updatedAt"`
}

func (r *AllowlistIdentifier) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk allowlist entry. When sign-ups are restricted to the allowlist, only allowlisted email addresses, phone numbers and web3 wallets can sign up. Entries cannot be updated: changing any input replaces the entry.")
}

func (a *AllowlistIdentifierArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Identifier, "The email address, phone number or web3 wallet to allow, for example an email address, a phone number, a web3 wallet or an email wildcard like *@example.com. Changing this forces a new resource.")
	ann.Describe(&a.Notify, "Whether to send an invitation email to the identifier when the entry is created. Only applies to email addresses. Clerk does not report this back, so it is only tracked from the program. Changing this forces a new resource.")
}

func (s *AllowlistIdentifierState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.IdentifierType, "The kind of identifier: email_address, phone_number or web3_wallet.")
	ann.Describe(&s.CreatedAt, "When the entry was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the entry was last updated (RFC 3339).")
}

var _ infer.CustomResource[AllowlistIdentifierArgs, AllowlistIdentifierState] = (*AllowlistIdentifier)(nil)
var _ infer.CustomRead[AllowlistIdentifierArgs, AllowlistIdentifierState] = (*AllowlistIdentifier)(nil)
var _ infer.CustomDelete[AllowlistIdentifierState] = (*AllowlistIdentifier)(nil)

// allowlistIdentifierJSON is the API shape.
type allowlistIdentifierJSON struct {
	ID             string `json:"id"`
	Identifier     string `json:"identifier"`
	IdentifierType string `json:"identifier_type"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (j allowlistIdentifierJSON) state() AllowlistIdentifierState {
	return AllowlistIdentifierState{
		AllowlistIdentifierArgs: AllowlistIdentifierArgs{Identifier: j.Identifier},
		IdentifierType:          j.IdentifierType,
		CreatedAt:               millisToRFC3339(j.CreatedAt),
		UpdatedAt:               millisToRFC3339(j.UpdatedAt),
	}
}

// findAllowlistIdentifier looks an entry up by ID or identifier; the API has no
// get-by-ID, so it reads the list.
func findAllowlistIdentifier(ctx context.Context, c *client, id, identifier string) (*allowlistIdentifierJSON, error) {
	items, err := listAll[allowlistIdentifierJSON](ctx, c, "/allowlist_identifiers", nil)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if (id != "" && items[i].ID == id) || (id == "" && items[i].Identifier == identifier) {
			return &items[i], nil
		}
	}
	return nil, nil
}

func (r *AllowlistIdentifier) Create(ctx context.Context, req infer.CreateRequest[AllowlistIdentifierArgs]) (infer.CreateResponse[AllowlistIdentifierState], error) {
	if req.DryRun {
		return infer.CreateResponse[AllowlistIdentifierState]{Output: AllowlistIdentifierState{AllowlistIdentifierArgs: req.Inputs}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[AllowlistIdentifierState]{}, err
	}
	body := map[string]any{"identifier": req.Inputs.Identifier}
	if req.Inputs.Notify != nil {
		body["notify"] = *req.Inputs.Notify
	}
	var out allowlistIdentifierJSON
	if err := c.do(ctx, http.MethodPost, "/allowlist_identifiers", body, &out); err != nil {
		return infer.CreateResponse[AllowlistIdentifierState]{}, fmt.Errorf("clerk: create allowlist identifier %q: %w", req.Inputs.Identifier, err)
	}
	st := out.state()
	st.Notify = req.Inputs.Notify
	return infer.CreateResponse[AllowlistIdentifierState]{ID: out.ID, Output: st}, nil
}

func (r *AllowlistIdentifier) Read(ctx context.Context, req infer.ReadRequest[AllowlistIdentifierArgs, AllowlistIdentifierState]) (infer.ReadResponse[AllowlistIdentifierArgs, AllowlistIdentifierState], error) {
	empty := infer.ReadResponse[AllowlistIdentifierArgs, AllowlistIdentifierState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := findAllowlistIdentifier(ctx, c, req.ID, "")
	if err != nil {
		return empty, fmt.Errorf("clerk: read allowlist identifier %q: %w", req.ID, err)
	}
	if got == nil {
		return empty, nil
	}
	st := got.state()
	// Clerk does not report notify; keep what the program asked for.
	st.Notify = req.Inputs.Notify
	return infer.ReadResponse[AllowlistIdentifierArgs, AllowlistIdentifierState]{ID: got.ID, Inputs: st.AllowlistIdentifierArgs, State: st}, nil
}

func (r *AllowlistIdentifier) Delete(ctx context.Context, req infer.DeleteRequest[AllowlistIdentifierState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/allowlist_identifiers/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete allowlist identifier %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetAllowlistIdentifier looks up an allowlist entry by ID or identifier.
type GetAllowlistIdentifier struct{}

// GetAllowlistIdentifierArgs selects the entry; set exactly one of the fields.
type GetAllowlistIdentifierArgs struct {
	ID         *string `pulumi:"allowlistIdentifierId,optional"`
	Identifier *string `pulumi:"identifier,optional"`
}

// GetAllowlistIdentifierResult is the looked-up entry.
type GetAllowlistIdentifierResult struct {
	ID             string `pulumi:"allowlistIdentifierId"`
	Identifier     string `pulumi:"identifier"`
	IdentifierType string `pulumi:"identifierType"`
	CreatedAt      string `pulumi:"createdAt"`
	UpdatedAt      string `pulumi:"updatedAt"`
}

func (f *GetAllowlistIdentifier) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk allowlist entry by ID or by identifier.")
}

func (a *GetAllowlistIdentifierArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The allowlist entry ID. Set either allowlistIdentifierId or identifier.")
	ann.Describe(&a.Identifier, "The allowlisted email address, phone number, web3 wallet or wildcard. Set either allowlistIdentifierId or identifier.")
}

func (f *GetAllowlistIdentifier) Invoke(ctx context.Context, req infer.FunctionRequest[GetAllowlistIdentifierArgs]) (infer.FunctionResponse[GetAllowlistIdentifierResult], error) {
	out := infer.FunctionResponse[GetAllowlistIdentifierResult]{}
	id, ident := derefStr(req.Input.ID), derefStr(req.Input.Identifier)
	if (id == "") == (ident == "") {
		return out, fmt.Errorf("clerk: getAllowlistIdentifier needs exactly one of allowlistIdentifierId or identifier")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	got, err := findAllowlistIdentifier(ctx, c, id, ident)
	if err != nil {
		return out, lookupErr("allowlist identifier", id+ident, err)
	}
	if got == nil {
		return out, lookupErr("allowlist identifier", fmt.Sprintf("%q", id+ident), &apiError{Status: http.StatusNotFound})
	}
	st := got.state()
	out.Output = GetAllowlistIdentifierResult{ID: got.ID, Identifier: got.Identifier, IdentifierType: st.IdentifierType, CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt}
	return out, nil
}
