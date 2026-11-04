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

// BlocklistIdentifier blocks an email address, phone number or web3 wallet
// from signing up to a Clerk instance.
type BlocklistIdentifier struct{}

// BlocklistIdentifierArgs are the inputs of a BlocklistIdentifier.
type BlocklistIdentifierArgs struct {
	// Identifier is the email address, phone number or web3 wallet to block.
	Identifier string `pulumi:"identifier" provider:"replaceOnChanges"`
}

// BlocklistIdentifierState is the stored state of a BlocklistIdentifier.
type BlocklistIdentifierState struct {
	BlocklistIdentifierArgs
	IdentifierType string `pulumi:"identifierType"`
	CreatedAt      string `pulumi:"createdAt"`
	UpdatedAt      string `pulumi:"updatedAt"`
}

func (r *BlocklistIdentifier) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk blocklist entry. Blocklisted email addresses, phone numbers and web3 wallets cannot be used to sign up. Entries cannot be updated: changing the identifier replaces the entry.")
}

func (a *BlocklistIdentifierArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Identifier, "The email address, phone number or web3 wallet to block. Changing this forces a new resource.")
}

func (s *BlocklistIdentifierState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.IdentifierType, "The kind of identifier: email_address, phone_number or web3_wallet.")
	ann.Describe(&s.CreatedAt, "When the entry was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the entry was last updated (RFC 3339).")
}

var _ infer.CustomResource[BlocklistIdentifierArgs, BlocklistIdentifierState] = (*BlocklistIdentifier)(nil)
var _ infer.CustomRead[BlocklistIdentifierArgs, BlocklistIdentifierState] = (*BlocklistIdentifier)(nil)
var _ infer.CustomDelete[BlocklistIdentifierState] = (*BlocklistIdentifier)(nil)

// blocklistIdentifierJSON is the API shape.
type blocklistIdentifierJSON struct {
	ID             string `json:"id"`
	Identifier     string `json:"identifier"`
	IdentifierType string `json:"identifier_type"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (j blocklistIdentifierJSON) state() BlocklistIdentifierState {
	return BlocklistIdentifierState{
		BlocklistIdentifierArgs: BlocklistIdentifierArgs{Identifier: j.Identifier},
		IdentifierType:          j.IdentifierType,
		CreatedAt:               millisToRFC3339(j.CreatedAt),
		UpdatedAt:               millisToRFC3339(j.UpdatedAt),
	}
}

// findBlocklistIdentifier looks an entry up by ID or identifier; the API has no
// get-by-ID, so it reads the list.
func findBlocklistIdentifier(ctx context.Context, c *client, id, identifier string) (*blocklistIdentifierJSON, error) {
	items, err := listAll[blocklistIdentifierJSON](ctx, c, "/blocklist_identifiers", nil)
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

func (r *BlocklistIdentifier) Create(ctx context.Context, req infer.CreateRequest[BlocklistIdentifierArgs]) (infer.CreateResponse[BlocklistIdentifierState], error) {
	if req.DryRun {
		return infer.CreateResponse[BlocklistIdentifierState]{Output: BlocklistIdentifierState{BlocklistIdentifierArgs: req.Inputs}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[BlocklistIdentifierState]{}, err
	}
	var out blocklistIdentifierJSON
	if err := c.do(ctx, http.MethodPost, "/blocklist_identifiers", map[string]any{"identifier": req.Inputs.Identifier}, &out); err != nil {
		return infer.CreateResponse[BlocklistIdentifierState]{}, fmt.Errorf("clerk: create blocklist identifier %q: %w", req.Inputs.Identifier, err)
	}
	return infer.CreateResponse[BlocklistIdentifierState]{ID: out.ID, Output: out.state()}, nil
}

func (r *BlocklistIdentifier) Read(ctx context.Context, req infer.ReadRequest[BlocklistIdentifierArgs, BlocklistIdentifierState]) (infer.ReadResponse[BlocklistIdentifierArgs, BlocklistIdentifierState], error) {
	empty := infer.ReadResponse[BlocklistIdentifierArgs, BlocklistIdentifierState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := findBlocklistIdentifier(ctx, c, req.ID, "")
	if err != nil {
		return empty, fmt.Errorf("clerk: read blocklist identifier %q: %w", req.ID, err)
	}
	if got == nil {
		return empty, nil
	}
	st := got.state()
	return infer.ReadResponse[BlocklistIdentifierArgs, BlocklistIdentifierState]{ID: got.ID, Inputs: st.BlocklistIdentifierArgs, State: st}, nil
}

func (r *BlocklistIdentifier) Delete(ctx context.Context, req infer.DeleteRequest[BlocklistIdentifierState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/blocklist_identifiers/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete blocklist identifier %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetBlocklistIdentifier looks up a blocklist entry by ID or identifier.
type GetBlocklistIdentifier struct{}

// GetBlocklistIdentifierArgs selects the entry; set exactly one of the fields.
type GetBlocklistIdentifierArgs struct {
	ID         *string `pulumi:"blocklistIdentifierId,optional"`
	Identifier *string `pulumi:"identifier,optional"`
}

// GetBlocklistIdentifierResult is the looked-up entry.
type GetBlocklistIdentifierResult struct {
	ID             string `pulumi:"blocklistIdentifierId"`
	Identifier     string `pulumi:"identifier"`
	IdentifierType string `pulumi:"identifierType"`
	CreatedAt      string `pulumi:"createdAt"`
	UpdatedAt      string `pulumi:"updatedAt"`
}

func (f *GetBlocklistIdentifier) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk blocklist entry by ID or by identifier.")
}

func (a *GetBlocklistIdentifierArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The blocklist entry ID. Set either blocklistIdentifierId or identifier.")
	ann.Describe(&a.Identifier, "The blocked email address, phone number or web3 wallet. Set either blocklistIdentifierId or identifier.")
}

func (f *GetBlocklistIdentifier) Invoke(ctx context.Context, req infer.FunctionRequest[GetBlocklistIdentifierArgs]) (infer.FunctionResponse[GetBlocklistIdentifierResult], error) {
	out := infer.FunctionResponse[GetBlocklistIdentifierResult]{}
	id, ident := derefStr(req.Input.ID), derefStr(req.Input.Identifier)
	if (id == "") == (ident == "") {
		return out, fmt.Errorf("clerk: getBlocklistIdentifier needs exactly one of blocklistIdentifierId or identifier")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	got, err := findBlocklistIdentifier(ctx, c, id, ident)
	if err != nil {
		return out, lookupErr("blocklist identifier", id+ident, err)
	}
	if got == nil {
		return out, lookupErr("blocklist identifier", fmt.Sprintf("%q", id+ident), &apiError{Status: http.StatusNotFound})
	}
	st := got.state()
	out.Output = GetBlocklistIdentifierResult{ID: got.ID, Identifier: got.Identifier, IdentifierType: st.IdentifierType, CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt}
	return out, nil
}
