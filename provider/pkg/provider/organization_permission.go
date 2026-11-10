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

// OrganizationPermission is a custom permission that organization roles can grant.
type OrganizationPermission struct{}

// OrganizationPermissionArgs are the inputs of an OrganizationPermission.
type OrganizationPermissionArgs struct {
	// Name is the display name.
	Name string `pulumi:"name"`
	// Key is the unique permission key, for example org:posts:create.
	Key string `pulumi:"key"`
	// Description says what the permission allows.
	Description *string `pulumi:"description,optional"`
}

// OrganizationPermissionState is the stored state of an OrganizationPermission.
type OrganizationPermissionState struct {
	OrganizationPermissionArgs
	Type      string `pulumi:"type"`
	CreatedAt string `pulumi:"createdAt"`
	UpdatedAt string `pulumi:"updatedAt"`
}

func (r *OrganizationPermission) Annotate(a infer.Annotator) {
	a.Describe(&r, "A custom Clerk organization permission. Grant it to members by listing its ID in an OrganizationRole. Import with the permission ID (perm_...).")
}

func (a *OrganizationPermissionArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "Display name of the permission.")
	ann.Describe(&a.Key, "Unique key for the permission, in the form org:<feature>:<action> (for example org:posts:create).")
	ann.Describe(&a.Description, "Description of what the permission allows.")
}

func (s *OrganizationPermissionState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.Type, "Type of the permission: system or user.")
	ann.Describe(&s.CreatedAt, "When the permission was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the permission was last updated (RFC 3339).")
}

var _ infer.CustomResource[OrganizationPermissionArgs, OrganizationPermissionState] = (*OrganizationPermission)(nil)
var _ infer.CustomRead[OrganizationPermissionArgs, OrganizationPermissionState] = (*OrganizationPermission)(nil)
var _ infer.CustomUpdate[OrganizationPermissionArgs, OrganizationPermissionState] = (*OrganizationPermission)(nil)
var _ infer.CustomDelete[OrganizationPermissionState] = (*OrganizationPermission)(nil)

// organizationPermissionJSON is the API shape.
type organizationPermissionJSON struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Key         string  `json:"key"`
	Description *string `json:"description"`
	Type        string  `json:"type"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

func (j organizationPermissionJSON) state(prev OrganizationPermissionArgs) OrganizationPermissionState {
	return OrganizationPermissionState{
		OrganizationPermissionArgs: OrganizationPermissionArgs{Name: j.Name, Key: j.Key, Description: keepStr(prev.Description, j.Description)},
		Type:                       j.Type,
		CreatedAt:                  millisToRFC3339(j.CreatedAt),
		UpdatedAt:                  millisToRFC3339(j.UpdatedAt),
	}
}

func (r *OrganizationPermission) Create(ctx context.Context, req infer.CreateRequest[OrganizationPermissionArgs]) (infer.CreateResponse[OrganizationPermissionState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[OrganizationPermissionState]{Output: OrganizationPermissionState{OrganizationPermissionArgs: in}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[OrganizationPermissionState]{}, err
	}
	body := map[string]any{"name": in.Name, "key": in.Key}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	var out organizationPermissionJSON
	if err := c.do(ctx, http.MethodPost, "/organization_permissions", body, &out); err != nil {
		return infer.CreateResponse[OrganizationPermissionState]{}, fmt.Errorf("clerk: create organization permission %q: %w", in.Key, err)
	}
	return infer.CreateResponse[OrganizationPermissionState]{ID: out.ID, Output: out.state(in)}, nil
}

func (r *OrganizationPermission) Read(ctx context.Context, req infer.ReadRequest[OrganizationPermissionArgs, OrganizationPermissionState]) (infer.ReadResponse[OrganizationPermissionArgs, OrganizationPermissionState], error) {
	empty := infer.ReadResponse[OrganizationPermissionArgs, OrganizationPermissionState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	var j organizationPermissionJSON
	if err := c.do(ctx, http.MethodGet, "/organization_permissions/"+esc(req.ID), nil, &j); err != nil {
		if isNotFound(err) {
			if err := notFoundOnImport("organization permission", req.ID, req.Inputs, req.State); err != nil {
				return empty, err
			}
			return empty, nil
		}
		return empty, fmt.Errorf("clerk: read organization permission %q: %w", req.ID, err)
	}
	st := j.state(req.Inputs)
	return infer.ReadResponse[OrganizationPermissionArgs, OrganizationPermissionState]{ID: j.ID, Inputs: st.OrganizationPermissionArgs, State: st}, nil
}

func (r *OrganizationPermission) Update(ctx context.Context, req infer.UpdateRequest[OrganizationPermissionArgs, OrganizationPermissionState]) (infer.UpdateResponse[OrganizationPermissionState], error) {
	in := req.Inputs
	if req.DryRun {
		st := req.State
		st.OrganizationPermissionArgs = in
		return infer.UpdateResponse[OrganizationPermissionState]{Output: st}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[OrganizationPermissionState]{}, err
	}
	body := map[string]any{"name": in.Name, "key": in.Key}
	if in.Description != nil {
		body["description"] = *in.Description
	} else if req.State.Description != nil {
		body["description"] = ""
	}
	var out organizationPermissionJSON
	if err := c.do(ctx, http.MethodPatch, "/organization_permissions/"+esc(req.ID), body, &out); err != nil {
		return infer.UpdateResponse[OrganizationPermissionState]{}, fmt.Errorf("clerk: update organization permission %q: %w", req.ID, err)
	}
	return infer.UpdateResponse[OrganizationPermissionState]{Output: out.state(in)}, nil
}

func (r *OrganizationPermission) Delete(ctx context.Context, req infer.DeleteRequest[OrganizationPermissionState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/organization_permissions/"+esc(req.ID), nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete organization permission %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetOrganizationPermission looks up an organization permission by ID or key.
type GetOrganizationPermission struct{}

// GetOrganizationPermissionArgs selects the permission; set exactly one of the fields.
type GetOrganizationPermissionArgs struct {
	ID  *string `pulumi:"organizationPermissionId,optional"`
	Key *string `pulumi:"key,optional"`
}

// GetOrganizationPermissionResult is the looked-up permission.
type GetOrganizationPermissionResult struct {
	ID          string `pulumi:"organizationPermissionId"`
	Name        string `pulumi:"name"`
	Key         string `pulumi:"key"`
	Description string `pulumi:"description"`
	Type        string `pulumi:"type"`
	CreatedAt   string `pulumi:"createdAt"`
	UpdatedAt   string `pulumi:"updatedAt"`
}

func (f *GetOrganizationPermission) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk organization permission by ID or by key.")
}

func (a *GetOrganizationPermissionArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The permission ID (perm_...). Set either id or key.")
	ann.Describe(&a.Key, "The permission key, for example org:posts:create. Set either id or key.")
}

func (f *GetOrganizationPermission) Invoke(ctx context.Context, req infer.FunctionRequest[GetOrganizationPermissionArgs]) (infer.FunctionResponse[GetOrganizationPermissionResult], error) {
	out := infer.FunctionResponse[GetOrganizationPermissionResult]{}
	id, key := derefStr(req.Input.ID), derefStr(req.Input.Key)
	if (id == "") == (key == "") {
		return out, fmt.Errorf("clerk: getOrganizationPermission needs exactly one of id or key")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	var found *organizationPermissionJSON
	if id != "" {
		var j organizationPermissionJSON
		if err := c.do(ctx, http.MethodGet, "/organization_permissions/"+esc(id), nil, &j); err != nil {
			return out, lookupErr("organization permission", fmt.Sprintf("%q", id), err)
		}
		found = &j
	} else {
		items, err := listAll[organizationPermissionJSON](ctx, c, "/organization_permissions", nil)
		if err != nil {
			return out, lookupErr("organization permission", fmt.Sprintf("%q", key), err)
		}
		for i := range items {
			if items[i].Key == key {
				found = &items[i]
				break
			}
		}
		if found == nil {
			return out, lookupErr("organization permission", fmt.Sprintf("%q", key), &apiError{Status: http.StatusNotFound})
		}
	}
	st := found.state(OrganizationPermissionArgs{})
	out.Output = GetOrganizationPermissionResult{
		ID: found.ID, Name: found.Name, Key: found.Key, Description: derefStr(found.Description),
		Type: found.Type, CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt,
	}
	return out, nil
}
