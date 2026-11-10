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

// OrganizationRole is a custom role for members of Clerk organizations.
type OrganizationRole struct{}

// OrganizationRoleArgs are the inputs of an OrganizationRole.
type OrganizationRoleArgs struct {
	// Name is the display name.
	Name string `pulumi:"name"`
	// Key is the unique role key, for example org:editor.
	Key string `pulumi:"key"`
	// Description describes the role.
	Description *string `pulumi:"description,optional"`
	// Permissions lists the IDs of the permissions granted by the role.
	Permissions []string `pulumi:"permissions,optional"`
}

// OrganizationRoleState is the stored state of an OrganizationRole.
type OrganizationRoleState struct {
	OrganizationRoleArgs
	IsCreatorEligible bool   `pulumi:"isCreatorEligible"`
	CreatedAt         string `pulumi:"createdAt"`
	UpdatedAt         string `pulumi:"updatedAt"`
}

func (r *OrganizationRole) Annotate(a infer.Annotator) {
	a.Describe(&r, "A custom Clerk organization role. Import with the role ID (role_...). The permission list is a set: order does not matter.")
}

func (a *OrganizationRoleArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "Display name of the role.")
	ann.Describe(&a.Key, "Unique key for the role, in the form org:<name> (for example org:editor).")
	ann.Describe(&a.Description, "Description of the role.")
	ann.Describe(&a.Permissions, "IDs of the permissions (perm_...) to grant. Order is not significant.")
}

func (s *OrganizationRoleState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.IsCreatorEligible, "Whether the role can be assigned to organization creators in a role set.")
	ann.Describe(&s.CreatedAt, "When the role was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the role was last updated (RFC 3339).")
}

var _ infer.CustomResource[OrganizationRoleArgs, OrganizationRoleState] = (*OrganizationRole)(nil)
var _ infer.CustomRead[OrganizationRoleArgs, OrganizationRoleState] = (*OrganizationRole)(nil)
var _ infer.CustomUpdate[OrganizationRoleArgs, OrganizationRoleState] = (*OrganizationRole)(nil)
var _ infer.CustomDiff[OrganizationRoleArgs, OrganizationRoleState] = (*OrganizationRole)(nil)
var _ infer.CustomDelete[OrganizationRoleState] = (*OrganizationRole)(nil)

// organizationRoleJSON is the API shape.
type organizationRoleJSON struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Key               string  `json:"key"`
	Description       *string `json:"description"`
	IsCreatorEligible bool    `json:"is_creator_eligible"`
	Permissions       []struct {
		ID string `json:"id"`
	} `json:"permissions"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// state maps the API object; prev keeps the user's description and
// permission order when they mean the same as what Clerk reports.
func (j organizationRoleJSON) state(prev OrganizationRoleArgs) OrganizationRoleState {
	var perms []string
	if len(j.Permissions) > 0 || prev.Permissions != nil {
		perms = make([]string, 0, len(j.Permissions))
		for _, p := range j.Permissions {
			perms = append(perms, p.ID)
		}
		perms = keepOrder(prev.Permissions, perms)
	}
	return OrganizationRoleState{
		OrganizationRoleArgs: OrganizationRoleArgs{Name: j.Name, Key: j.Key, Description: keepStr(prev.Description, j.Description), Permissions: perms},
		IsCreatorEligible:    j.IsCreatorEligible,
		CreatedAt:            millisToRFC3339(j.CreatedAt),
		UpdatedAt:            millisToRFC3339(j.UpdatedAt),
	}
}

func (r *OrganizationRole) Diff(_ context.Context, req infer.DiffRequest[OrganizationRoleArgs, OrganizationRoleState]) (infer.DiffResponse, error) {
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
	if !sameSet(in.Permissions, old.Permissions) {
		d.update("permissions")
	}
	return d.resp, nil
}

func (r *OrganizationRole) Create(ctx context.Context, req infer.CreateRequest[OrganizationRoleArgs]) (infer.CreateResponse[OrganizationRoleState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[OrganizationRoleState]{Output: OrganizationRoleState{OrganizationRoleArgs: in}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[OrganizationRoleState]{}, err
	}
	body := map[string]any{"name": in.Name, "key": in.Key}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	if in.Permissions != nil {
		body["permissions"] = in.Permissions
	}
	var out organizationRoleJSON
	if err := c.do(ctx, http.MethodPost, "/organization_roles", body, &out); err != nil {
		return infer.CreateResponse[OrganizationRoleState]{}, fmt.Errorf("clerk: create organization role %q: %w", in.Key, err)
	}
	return infer.CreateResponse[OrganizationRoleState]{ID: out.ID, Output: out.state(in)}, nil
}

func (r *OrganizationRole) Read(ctx context.Context, req infer.ReadRequest[OrganizationRoleArgs, OrganizationRoleState]) (infer.ReadResponse[OrganizationRoleArgs, OrganizationRoleState], error) {
	empty := infer.ReadResponse[OrganizationRoleArgs, OrganizationRoleState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	var j organizationRoleJSON
	if err := c.do(ctx, http.MethodGet, "/organization_roles/"+esc(req.ID), nil, &j); err != nil {
		if isNotFound(err) {
			if err := notFoundOnImport("organization role", req.ID, req.Inputs, req.State); err != nil {
				return empty, err
			}
			return empty, nil
		}
		return empty, fmt.Errorf("clerk: read organization role %q: %w", req.ID, err)
	}
	st := j.state(req.Inputs)
	return infer.ReadResponse[OrganizationRoleArgs, OrganizationRoleState]{ID: j.ID, Inputs: st.OrganizationRoleArgs, State: st}, nil
}

func (r *OrganizationRole) Update(ctx context.Context, req infer.UpdateRequest[OrganizationRoleArgs, OrganizationRoleState]) (infer.UpdateResponse[OrganizationRoleState], error) {
	in := req.Inputs
	if req.DryRun {
		st := req.State
		st.OrganizationRoleArgs = in
		return infer.UpdateResponse[OrganizationRoleState]{Output: st}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[OrganizationRoleState]{}, err
	}
	body := map[string]any{"name": in.Name, "key": in.Key}
	if in.Description != nil {
		body["description"] = *in.Description
	} else if req.State.Description != nil {
		body["description"] = ""
	}
	if in.Permissions != nil {
		body["permissions"] = in.Permissions
	} else if len(req.State.Permissions) > 0 {
		body["permissions"] = []string{}
	}
	var out organizationRoleJSON
	if err := c.do(ctx, http.MethodPatch, "/organization_roles/"+esc(req.ID), body, &out); err != nil {
		return infer.UpdateResponse[OrganizationRoleState]{}, fmt.Errorf("clerk: update organization role %q: %w", req.ID, err)
	}
	return infer.UpdateResponse[OrganizationRoleState]{Output: out.state(in)}, nil
}

func (r *OrganizationRole) Delete(ctx context.Context, req infer.DeleteRequest[OrganizationRoleState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/organization_roles/"+esc(req.ID), nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete organization role %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetOrganizationRole looks up an organization role by ID or key.
type GetOrganizationRole struct{}

// GetOrganizationRoleArgs selects the role; set exactly one of the fields.
type GetOrganizationRoleArgs struct {
	ID  *string `pulumi:"organizationRoleId,optional"`
	Key *string `pulumi:"key,optional"`
}

// GetOrganizationRoleResult is the looked-up role.
type GetOrganizationRoleResult struct {
	ID                string   `pulumi:"organizationRoleId"`
	Name              string   `pulumi:"name"`
	Key               string   `pulumi:"key"`
	Description       string   `pulumi:"description"`
	Permissions       []string `pulumi:"permissions"`
	IsCreatorEligible bool     `pulumi:"isCreatorEligible"`
	CreatedAt         string   `pulumi:"createdAt"`
	UpdatedAt         string   `pulumi:"updatedAt"`
}

func (f *GetOrganizationRole) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk organization role by ID or by key.")
}

func (a *GetOrganizationRoleArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The role ID (role_...). Set either id or key.")
	ann.Describe(&a.Key, "The role key, for example org:admin. Set either id or key.")
}

func (f *GetOrganizationRole) Invoke(ctx context.Context, req infer.FunctionRequest[GetOrganizationRoleArgs]) (infer.FunctionResponse[GetOrganizationRoleResult], error) {
	out := infer.FunctionResponse[GetOrganizationRoleResult]{}
	id, key := derefStr(req.Input.ID), derefStr(req.Input.Key)
	if (id == "") == (key == "") {
		return out, fmt.Errorf("clerk: getOrganizationRole needs exactly one of id or key")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	var found *organizationRoleJSON
	if id != "" {
		var j organizationRoleJSON
		if err := c.do(ctx, http.MethodGet, "/organization_roles/"+esc(id), nil, &j); err != nil {
			return out, lookupErr("organization role", fmt.Sprintf("%q", id), err)
		}
		found = &j
	} else {
		items, err := listAll[organizationRoleJSON](ctx, c, "/organization_roles", nil)
		if err != nil {
			return out, lookupErr("organization role", fmt.Sprintf("%q", key), err)
		}
		for i := range items {
			if items[i].Key == key {
				found = &items[i]
				break
			}
		}
		if found == nil {
			return out, lookupErr("organization role", fmt.Sprintf("%q", key), &apiError{Status: http.StatusNotFound})
		}
	}
	st := found.state(OrganizationRoleArgs{})
	out.Output = GetOrganizationRoleResult{
		ID: found.ID, Name: found.Name, Key: found.Key, Description: derefStr(found.Description),
		Permissions: append([]string{}, st.Permissions...), IsCreatorEligible: found.IsCreatorEligible,
		CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt,
	}
	return out, nil
}
