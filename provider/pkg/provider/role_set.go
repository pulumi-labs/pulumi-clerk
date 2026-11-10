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

// RoleSet is a named collection of organization roles.
type RoleSet struct{}

// RoleSetArgs are the inputs of a RoleSet.
type RoleSetArgs struct {
	// Name is the display name.
	Name string `pulumi:"name"`
	// Key is the unique role set key, for example role_set:default.
	Key string `pulumi:"key"`
	// Description describes the role set.
	Description *string `pulumi:"description,optional"`
	// Type is initial or custom.
	Type *string `pulumi:"type,optional"`
	// DefaultRoleKey is assigned to new members.
	DefaultRoleKey string `pulumi:"defaultRoleKey"`
	// CreatorRoleKey is assigned to organization creators.
	CreatorRoleKey string `pulumi:"creatorRoleKey"`
	// Roles lists the role keys in the set.
	Roles []string `pulumi:"roles"`
}

// RoleSetState is the stored state of a RoleSet.
type RoleSetState struct {
	RoleSetArgs
	CreatedAt string `pulumi:"createdAt"`
	UpdatedAt string `pulumi:"updatedAt"`
}

func (r *RoleSet) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk role set: the collection of roles an organization can use, with the role given to new members and the role given to the creator. Import with the role set ID (rset_...). The role list is a set: order does not matter.")
}

func (a *RoleSetArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "Display name of the role set.")
	ann.Describe(&a.Key, "Unique key for the role set. Must start with role_set: and use only lowercase letters, digits and underscores.")
	ann.Describe(&a.Description, "Description of the role set.")
	ann.Describe(&a.Type, "Type of the role set: initial (the default set for new organizations; only one per instance) or custom. Defaults to custom.")
	ann.Describe(&a.DefaultRoleKey, "Key of the role automatically assigned to new organization members. Must be in roles.")
	ann.Describe(&a.CreatorRoleKey, "Key of the role assigned to the creator of an organization. Must be in roles.")
	ann.Describe(&a.Roles, "Role keys to include (1 to 10). Must contain defaultRoleKey and creatorRoleKey. Order is not significant.")
}

func (s *RoleSetState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.CreatedAt, "When the role set was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the role set was last updated (RFC 3339).")
}

var _ infer.CustomResource[RoleSetArgs, RoleSetState] = (*RoleSet)(nil)
var _ infer.CustomRead[RoleSetArgs, RoleSetState] = (*RoleSet)(nil)
var _ infer.CustomUpdate[RoleSetArgs, RoleSetState] = (*RoleSet)(nil)
var _ infer.CustomDiff[RoleSetArgs, RoleSetState] = (*RoleSet)(nil)
var _ infer.CustomDelete[RoleSetState] = (*RoleSet)(nil)

type roleSetItemJSON struct {
	Key string `json:"key"`
}

// roleSetJSON is the API shape.
type roleSetJSON struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Key         string            `json:"key"`
	Description *string           `json:"description"`
	Roles       []roleSetItemJSON `json:"roles"`
	DefaultRole *roleSetItemJSON  `json:"default_role"`
	CreatorRole *roleSetItemJSON  `json:"creator_role"`
	Type        string            `json:"type"`
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
}

func (j roleSetJSON) state(prev RoleSetArgs) RoleSetState {
	keys := make([]string, 0, len(j.Roles))
	for _, r := range j.Roles {
		keys = append(keys, r.Key)
	}
	def, creator := prev.DefaultRoleKey, prev.CreatorRoleKey
	if j.DefaultRole != nil {
		def = j.DefaultRole.Key
	}
	if j.CreatorRole != nil {
		creator = j.CreatorRole.Key
	}
	st := RoleSetState{
		RoleSetArgs: RoleSetArgs{
			Name: j.Name, Key: j.Key, Description: keepStr(prev.Description, j.Description),
			DefaultRoleKey: def, CreatorRoleKey: creator, Roles: keepOrder(prev.Roles, keys),
		},
		CreatedAt: millisToRFC3339(j.CreatedAt),
		UpdatedAt: millisToRFC3339(j.UpdatedAt),
	}
	if j.Type != "" {
		st.Type = strPtr(j.Type)
	}
	return st
}

func roleSetInputs(prior RoleSetArgs, st RoleSetState, imported bool) RoleSetArgs {
	in := st.RoleSetArgs
	in.Type = nil
	if imported || prior.Type != nil {
		in.Type = st.Type
	}
	return in
}

// roleSetPreview is the predicted state of an update.
func roleSetPreview(in RoleSetArgs, old RoleSetState) RoleSetState {
	st := old
	typ := st.Type
	st.RoleSetArgs = in
	if in.Type == nil {
		st.Type = typ
	}
	return st
}

func (r *RoleSet) Diff(_ context.Context, req infer.DiffRequest[RoleSetArgs, RoleSetState]) (infer.DiffResponse, error) {
	in, old := req.Inputs, req.State
	var d diffAcc
	if in.Name != old.Name {
		d.update("name")
	}
	if in.Key != old.Key {
		d.update("key")
	}
	if derefStr(in.Description) != derefStr(old.Description) {
		d.update("description")
	}
	if in.Type != nil && derefStr(in.Type) != derefStr(old.Type) {
		d.update("type")
	}
	if in.DefaultRoleKey != old.DefaultRoleKey {
		d.update("defaultRoleKey")
	}
	if in.CreatorRoleKey != old.CreatorRoleKey {
		d.update("creatorRoleKey")
	}
	if !sameSet(in.Roles, old.Roles) {
		d.update("roles")
	}
	return d.resp, nil
}

func (r *RoleSet) Create(ctx context.Context, req infer.CreateRequest[RoleSetArgs]) (infer.CreateResponse[RoleSetState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[RoleSetState]{Output: RoleSetState{RoleSetArgs: in}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[RoleSetState]{}, err
	}
	body := map[string]any{
		"name": in.Name, "key": in.Key, "default_role_key": in.DefaultRoleKey,
		"creator_role_key": in.CreatorRoleKey, "roles": in.Roles,
	}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	if in.Type != nil {
		body["type"] = *in.Type
	}
	var out roleSetJSON
	if err := c.do(ctx, http.MethodPost, "/role_sets", body, &out); err != nil {
		return infer.CreateResponse[RoleSetState]{}, fmt.Errorf("clerk: create role set %q: %w", in.Key, err)
	}
	return infer.CreateResponse[RoleSetState]{ID: out.ID, Output: out.state(in)}, nil
}

func (r *RoleSet) Read(ctx context.Context, req infer.ReadRequest[RoleSetArgs, RoleSetState]) (infer.ReadResponse[RoleSetArgs, RoleSetState], error) {
	empty := infer.ReadResponse[RoleSetArgs, RoleSetState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	var j roleSetJSON
	if err := c.do(ctx, http.MethodGet, "/role_sets/"+esc(req.ID), nil, &j); err != nil {
		if isNotFound(err) {
			if err := notFoundOnImport("role set", req.ID, req.Inputs, req.State); err != nil {
				return empty, err
			}
			return empty, nil
		}
		return empty, fmt.Errorf("clerk: read role set %q: %w", req.ID, err)
	}
	imported := req.Inputs.Name == "" && req.State.Name == ""
	st := j.state(req.State.RoleSetArgs)
	return infer.ReadResponse[RoleSetArgs, RoleSetState]{ID: j.ID, Inputs: roleSetInputs(req.Inputs, st, imported), State: st}, nil
}

// Update applies a role set change in an order Clerk accepts: new roles first
// (a new default or creator role must already be in the set), then the
// scalar fields and the default and creator keys, then role removals (a
// removed role must no longer be the default or creator).
func (r *RoleSet) Update(ctx context.Context, req infer.UpdateRequest[RoleSetArgs, RoleSetState]) (infer.UpdateResponse[RoleSetState], error) {
	in, old := req.Inputs, req.State
	if req.DryRun {
		return infer.UpdateResponse[RoleSetState]{Output: roleSetPreview(in, old)}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[RoleSetState]{}, err
	}
	path := "/role_sets/" + esc(req.ID)
	// partial reports a failure after earlier steps already changed the role
	// set: the best-known state is returned so the next update retries.
	partial := func(step string, cause error) (infer.UpdateResponse[RoleSetState], error) {
		st := old
		var cur roleSetJSON
		if err := c.do(ctx, http.MethodGet, path, nil, &cur); err == nil {
			st = cur.state(old.RoleSetArgs)
		}
		return infer.UpdateResponse[RoleSetState]{Output: st},
			infer.ResourceInitFailedError{Reasons: []string{fmt.Sprintf("role set %s partially updated: %s failed: %v", req.ID, step, cause)}}
	}
	have := map[string]bool{}
	for _, k := range old.Roles {
		have[k] = true
	}
	want := map[string]bool{}
	var add, remove []string
	for _, k := range in.Roles {
		want[k] = true
		if !have[k] {
			add = append(add, k)
		}
	}
	for _, k := range old.Roles {
		if !want[k] {
			remove = append(remove, k)
		}
	}
	var last roleSetJSON
	didSomething := false
	if len(add) > 0 {
		if err := c.do(ctx, http.MethodPost, path+"/roles", map[string]any{"role_keys": add}, &last); err != nil {
			return infer.UpdateResponse[RoleSetState]{}, fmt.Errorf("clerk: add roles to role set %q: %w", req.ID, err)
		}
		didSomething = true
	}
	patch := map[string]any{}
	if in.Name != old.Name {
		patch["name"] = in.Name
	}
	if in.Key != old.Key {
		patch["key"] = in.Key
	}
	if derefStr(in.Description) != derefStr(old.Description) {
		patch["description"] = derefStr(in.Description)
	}
	if in.Type != nil && derefStr(in.Type) != derefStr(old.Type) {
		patch["type"] = *in.Type
	}
	if in.DefaultRoleKey != old.DefaultRoleKey {
		patch["default_role_key"] = in.DefaultRoleKey
	}
	if in.CreatorRoleKey != old.CreatorRoleKey {
		patch["creator_role_key"] = in.CreatorRoleKey
	}
	if len(patch) > 0 {
		if err := c.do(ctx, http.MethodPatch, path, patch, &last); err != nil {
			if didSomething {
				return partial("update", err)
			}
			return infer.UpdateResponse[RoleSetState]{}, fmt.Errorf("clerk: update role set %q: %w", req.ID, err)
		}
		didSomething = true
	}
	if len(remove) > 0 {
		if err := c.do(ctx, http.MethodDelete, path+"/roles", map[string]any{"role_keys": remove}, &last); err != nil {
			if didSomething {
				return partial("removing roles", err)
			}
			return infer.UpdateResponse[RoleSetState]{}, fmt.Errorf("clerk: remove roles from role set %q: %w", req.ID, err)
		}
		didSomething = true
	}
	if !didSomething || last.ID == "" {
		if err := c.do(ctx, http.MethodGet, path, nil, &last); err != nil {
			return infer.UpdateResponse[RoleSetState]{}, fmt.Errorf("clerk: read role set %q: %w", req.ID, err)
		}
	}
	return infer.UpdateResponse[RoleSetState]{Output: last.state(in)}, nil
}

func (r *RoleSet) Delete(ctx context.Context, req infer.DeleteRequest[RoleSetState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/role_sets/"+esc(req.ID), nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete role set %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetRoleSet looks up a role set by ID or key.
type GetRoleSet struct{}

// GetRoleSetArgs selects the role set; set exactly one of the fields.
type GetRoleSetArgs struct {
	ID  *string `pulumi:"roleSetId,optional"`
	Key *string `pulumi:"key,optional"`
}

// GetRoleSetResult is the looked-up role set.
type GetRoleSetResult struct {
	ID             string   `pulumi:"roleSetId"`
	Name           string   `pulumi:"name"`
	Key            string   `pulumi:"key"`
	Description    string   `pulumi:"description"`
	Type           string   `pulumi:"type"`
	DefaultRoleKey string   `pulumi:"defaultRoleKey"`
	CreatorRoleKey string   `pulumi:"creatorRoleKey"`
	Roles          []string `pulumi:"roles"`
	CreatedAt      string   `pulumi:"createdAt"`
	UpdatedAt      string   `pulumi:"updatedAt"`
}

func (f *GetRoleSet) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk role set by ID or by key.")
}

func (a *GetRoleSetArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The role set ID (rset_...). Set either id or key.")
	ann.Describe(&a.Key, "The role set key, for example role_set:default. Set either id or key.")
}

func (f *GetRoleSet) Invoke(ctx context.Context, req infer.FunctionRequest[GetRoleSetArgs]) (infer.FunctionResponse[GetRoleSetResult], error) {
	out := infer.FunctionResponse[GetRoleSetResult]{}
	id, key := derefStr(req.Input.ID), derefStr(req.Input.Key)
	if (id == "") == (key == "") {
		return out, fmt.Errorf("clerk: getRoleSet needs exactly one of id or key")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	// The API accepts a role set key or ID in the same path position.
	var j roleSetJSON
	if err := c.do(ctx, http.MethodGet, "/role_sets/"+esc(id+key), nil, &j); err != nil {
		return out, lookupErr("role set", fmt.Sprintf("%q", id+key), err)
	}
	st := j.state(RoleSetArgs{})
	out.Output = GetRoleSetResult{
		ID: j.ID, Name: j.Name, Key: j.Key, Description: derefStr(j.Description), Type: j.Type,
		DefaultRoleKey: st.DefaultRoleKey, CreatorRoleKey: st.CreatorRoleKey, Roles: st.Roles,
		CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt,
	}
	return out, nil
}
