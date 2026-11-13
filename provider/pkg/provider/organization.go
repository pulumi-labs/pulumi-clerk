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
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Organization is a Clerk organization.
type Organization struct{}

// OrganizationArgs are the inputs of an Organization.
type OrganizationArgs struct {
	// Name is the display name.
	Name string `pulumi:"name"`
	// Slug is the URL-friendly identifier; Clerk derives one from the name when unset.
	Slug *string `pulumi:"slug,optional"`
	// CreatedBy is the user who becomes the organization's first admin.
	CreatedBy *string `pulumi:"createdBy,optional" provider:"replaceOnChanges"`
	// MaxAllowedMemberships caps the member count.
	MaxAllowedMemberships *int `pulumi:"maxAllowedMemberships,optional"`
	// AdminDeleteEnabled lets organization admins delete the organization.
	AdminDeleteEnabled *bool `pulumi:"adminDeleteEnabled,optional"`
	// PublicMetadata is a JSON object readable from the frontend and backend.
	PublicMetadata *string `pulumi:"publicMetadata,optional"`
	// PrivateMetadata is a JSON object readable only from the backend.
	PrivateMetadata *string `pulumi:"privateMetadata,optional" provider:"secret"`
}

// OrganizationState is the stored state of an Organization.
type OrganizationState struct {
	OrganizationArgs
	CreatedAt string `pulumi:"createdAt"`
	UpdatedAt string `pulumi:"updatedAt"`
}

func (r *Organization) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk organization. Import with the organization ID (org_...). Requires the instance secret key; the slug only works when organization slugs are enabled in the Clerk Dashboard.")
}

func (a *OrganizationArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "The display name of the organization.")
	ann.Describe(&a.Slug, "URL-friendly identifier. Generated from the name when unset. Requires \"Enable organization slugs\" in the Clerk Dashboard (Configure > Organization settings).")
	ann.Describe(&a.CreatedBy, "ID of the user who creates the organization and becomes its admin. Only settable at creation; changing it forces a new resource.")
	ann.Describe(&a.MaxAllowedMemberships, "Maximum number of memberships allowed. Defaults to the instance setting when unset.")
	ann.Describe(&a.AdminDeleteEnabled, "Whether organization admins can delete the organization. Defaults to the instance setting when unset.")
	ann.Describe(&a.PublicMetadata, "Public metadata as a JSON object string, visible to the frontend and backend. Key order and whitespace do not matter.")
	ann.Describe(&a.PrivateMetadata, "Private metadata as a JSON object string, visible only to the backend. Key order and whitespace do not matter.")
}

func (s *OrganizationState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.CreatedAt, "When the organization was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the organization was last updated (RFC 3339).")
}

var _ infer.CustomResource[OrganizationArgs, OrganizationState] = (*Organization)(nil)
var _ infer.CustomRead[OrganizationArgs, OrganizationState] = (*Organization)(nil)
var _ infer.CustomUpdate[OrganizationArgs, OrganizationState] = (*Organization)(nil)
var _ infer.CustomDiff[OrganizationArgs, OrganizationState] = (*Organization)(nil)
var _ infer.CustomDelete[OrganizationState] = (*Organization)(nil)

// organizationJSON is the API shape.
type organizationJSON struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	Slug                  string          `json:"slug"`
	MaxAllowedMemberships int64           `json:"max_allowed_memberships"`
	AdminDeleteEnabled    bool            `json:"admin_delete_enabled"`
	PublicMetadata        json.RawMessage `json:"public_metadata"`
	PrivateMetadata       json.RawMessage `json:"private_metadata"`
	CreatedBy             string          `json:"created_by"`
	CreatedAt             int64           `json:"created_at"`
	UpdatedAt             int64           `json:"updated_at"`
}

// state maps the API object to state; prev supplies the user's previous
// metadata strings and created-by value.
func (j organizationJSON) state(prev OrganizationArgs) OrganizationState {
	max := int(j.MaxAllowedMemberships)
	admin := j.AdminDeleteEnabled
	st := OrganizationState{
		OrganizationArgs: OrganizationArgs{
			Name:                  j.Name,
			MaxAllowedMemberships: &max,
			AdminDeleteEnabled:    &admin,
			PublicMetadata:        keepJSON(prev.PublicMetadata, j.PublicMetadata),
			PrivateMetadata:       keepJSON(prev.PrivateMetadata, j.PrivateMetadata),
			CreatedBy:             prev.CreatedBy,
		},
		CreatedAt: millisToRFC3339(j.CreatedAt),
		UpdatedAt: millisToRFC3339(j.UpdatedAt),
	}
	if j.Slug != "" {
		st.Slug = strPtr(j.Slug)
	}
	if j.CreatedBy != "" {
		st.CreatedBy = strPtr(j.CreatedBy)
	}
	return st
}

// organizationInputs derives the inputs Read reports. On import there are no
// prior inputs, so everything Clerk reports is adopted; on refresh only the
// fields the user set are refreshed, so unset ones never become drift.
func organizationInputs(prior OrganizationArgs, st OrganizationState, imported bool) OrganizationArgs {
	in := OrganizationArgs{Name: st.Name}
	if imported {
		in.Slug, in.MaxAllowedMemberships, in.AdminDeleteEnabled = st.Slug, st.MaxAllowedMemberships, st.AdminDeleteEnabled
		in.PublicMetadata, in.PrivateMetadata = st.PublicMetadata, st.PrivateMetadata
		return in
	}
	if prior.Slug != nil {
		in.Slug = st.Slug
	}
	if prior.CreatedBy != nil {
		in.CreatedBy = st.CreatedBy
	}
	if prior.MaxAllowedMemberships != nil {
		in.MaxAllowedMemberships = st.MaxAllowedMemberships
	}
	if prior.AdminDeleteEnabled != nil {
		in.AdminDeleteEnabled = st.AdminDeleteEnabled
	}
	if prior.PublicMetadata != nil {
		in.PublicMetadata = st.PublicMetadata
	}
	if prior.PrivateMetadata != nil {
		in.PrivateMetadata = st.PrivateMetadata
	}
	return in
}

func organizationBody(in OrganizationArgs, create bool) map[string]any {
	b := map[string]any{"name": in.Name}
	if in.Slug != nil {
		b["slug"] = *in.Slug
	}
	if create && in.CreatedBy != nil {
		b["created_by"] = *in.CreatedBy
	}
	if in.MaxAllowedMemberships != nil {
		b["max_allowed_memberships"] = *in.MaxAllowedMemberships
	}
	if !create && in.AdminDeleteEnabled != nil {
		b["admin_delete_enabled"] = *in.AdminDeleteEnabled
	}
	// Clerk rejects metadata on PATCH /organizations/{id} (422
	// form_param_deprecated, API version 2026-05-12): an update sends it to
	// /organizations/{id}/metadata instead (see organizationMetadataUpdates).
	if create && in.PublicMetadata != nil {
		b["public_metadata"] = rawJSON(*in.PublicMetadata)
	}
	if create && in.PrivateMetadata != nil {
		b["private_metadata"] = rawJSON(*in.PrivateMetadata)
	}
	return b
}

// organizationMetadataUpdates returns one PUT body per metadata field whose
// declared value differs from the stored one (compared as JSON). PUT replaces
// the field entirely, so a key removed from the program disappears in Clerk
// (PATCH would deep-merge and keep it). Unchanged and unset fields are not sent.
func organizationMetadataUpdates(in OrganizationArgs, old OrganizationState) []map[string]any {
	var out []map[string]any
	if in.PublicMetadata != nil && canonJSON(*in.PublicMetadata) != canonOrEmpty(old.PublicMetadata) {
		out = append(out, map[string]any{"public_metadata": rawJSON(*in.PublicMetadata)})
	}
	if in.PrivateMetadata != nil && canonJSON(*in.PrivateMetadata) != canonOrEmpty(old.PrivateMetadata) {
		out = append(out, map[string]any{"private_metadata": rawJSON(*in.PrivateMetadata)})
	}
	return out
}

// organizationPreview is the predicted state of an update.
func organizationPreview(in OrganizationArgs, old OrganizationState) OrganizationState {
	st := old
	st.Name = in.Name
	if in.Slug != nil {
		st.Slug = in.Slug
	}
	if in.MaxAllowedMemberships != nil {
		st.MaxAllowedMemberships = in.MaxAllowedMemberships
	}
	if in.AdminDeleteEnabled != nil {
		st.AdminDeleteEnabled = in.AdminDeleteEnabled
	}
	if in.PublicMetadata != nil {
		st.PublicMetadata = in.PublicMetadata
	}
	if in.PrivateMetadata != nil {
		st.PrivateMetadata = in.PrivateMetadata
	}
	return st
}

// Diff treats an unset optional input as "leave whatever Clerk chose", and
// compares metadata as JSON rather than text.
func (r *Organization) Diff(_ context.Context, req infer.DiffRequest[OrganizationArgs, OrganizationState]) (infer.DiffResponse, error) {
	in, old := req.Inputs, req.State
	var d diffAcc
	if in.Name != old.Name {
		d.update("name")
	}
	if in.Slug != nil && derefStr(in.Slug) != derefStr(old.Slug) {
		d.update("slug")
	}
	if in.CreatedBy != nil && derefStr(in.CreatedBy) != derefStr(old.CreatedBy) {
		d.replace("createdBy")
	}
	if in.MaxAllowedMemberships != nil && (old.MaxAllowedMemberships == nil || *in.MaxAllowedMemberships != *old.MaxAllowedMemberships) {
		d.update("maxAllowedMemberships")
	}
	if in.AdminDeleteEnabled != nil && (old.AdminDeleteEnabled == nil || *in.AdminDeleteEnabled != *old.AdminDeleteEnabled) {
		d.update("adminDeleteEnabled")
	}
	if in.PublicMetadata != nil && canonJSON(*in.PublicMetadata) != canonOrEmpty(old.PublicMetadata) {
		d.update("publicMetadata")
	}
	if in.PrivateMetadata != nil && canonJSON(*in.PrivateMetadata) != canonOrEmpty(old.PrivateMetadata) {
		d.update("privateMetadata")
	}
	return d.resp, nil
}

func (r *Organization) Create(ctx context.Context, req infer.CreateRequest[OrganizationArgs]) (infer.CreateResponse[OrganizationState], error) {
	if req.DryRun {
		return infer.CreateResponse[OrganizationState]{Output: OrganizationState{OrganizationArgs: req.Inputs}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[OrganizationState]{}, err
	}
	var out organizationJSON
	if err := c.do(ctx, http.MethodPost, "/organizations", organizationBody(req.Inputs, true), &out); err != nil {
		return infer.CreateResponse[OrganizationState]{}, fmt.Errorf("clerk: create organization %q: %w", req.Inputs.Name, err)
	}
	// The create endpoint cannot set adminDeleteEnabled; follow up with an update.
	if want := req.Inputs.AdminDeleteEnabled; want != nil && *want != out.AdminDeleteEnabled {
		var upd organizationJSON
		if err := c.do(ctx, http.MethodPatch, "/organizations/"+esc(out.ID), map[string]any{"admin_delete_enabled": *want}, &upd); err != nil {
			return infer.CreateResponse[OrganizationState]{ID: out.ID, Output: out.state(req.Inputs)},
				infer.ResourceInitFailedError{Reasons: []string{fmt.Sprintf("organization %s created but setting adminDeleteEnabled failed: %v", out.ID, err)}}
		}
		out = upd
	}
	return infer.CreateResponse[OrganizationState]{ID: out.ID, Output: out.state(req.Inputs)}, nil
}

func (r *Organization) Read(ctx context.Context, req infer.ReadRequest[OrganizationArgs, OrganizationState]) (infer.ReadResponse[OrganizationArgs, OrganizationState], error) {
	empty := infer.ReadResponse[OrganizationArgs, OrganizationState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	var j organizationJSON
	if err := c.do(ctx, http.MethodGet, "/organizations/"+esc(req.ID), nil, &j); err != nil {
		if isNotFound(err) {
			if err := notFoundOnImport("organization", req.ID, req.Inputs, req.State); err != nil {
				return empty, err
			}
			return empty, nil
		}
		return empty, fmt.Errorf("clerk: read organization %q: %w", req.ID, err)
	}
	imported := req.Inputs.Name == "" && req.State.Name == ""
	st := j.state(req.State.OrganizationArgs)
	return infer.ReadResponse[OrganizationArgs, OrganizationState]{ID: j.ID, Inputs: organizationInputs(req.Inputs, st, imported), State: st}, nil
}

func (r *Organization) Update(ctx context.Context, req infer.UpdateRequest[OrganizationArgs, OrganizationState]) (infer.UpdateResponse[OrganizationState], error) {
	if req.DryRun {
		return infer.UpdateResponse[OrganizationState]{Output: organizationPreview(req.Inputs, req.State)}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[OrganizationState]{}, err
	}
	var out organizationJSON
	if err := c.do(ctx, http.MethodPatch, "/organizations/"+esc(req.ID), organizationBody(req.Inputs, false), &out); err != nil {
		return infer.UpdateResponse[OrganizationState]{}, fmt.Errorf("clerk: update organization %q: %w", req.ID, err)
	}
	for _, mb := range organizationMetadataUpdates(req.Inputs, req.State) {
		if err := c.do(ctx, http.MethodPut, "/organizations/"+esc(req.ID)+"/metadata", mb, &out); err != nil {
			return infer.UpdateResponse[OrganizationState]{}, fmt.Errorf("clerk: update metadata of organization %q: %w", req.ID, err)
		}
	}
	return infer.UpdateResponse[OrganizationState]{Output: out.state(req.Inputs)}, nil
}

func (r *Organization) Delete(ctx context.Context, req infer.DeleteRequest[OrganizationState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/organizations/"+esc(req.ID), nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete organization %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetOrganization looks up an organization by ID or slug.
type GetOrganization struct{}

// GetOrganizationArgs selects the organization; set exactly one of the fields.
type GetOrganizationArgs struct {
	ID   *string `pulumi:"organizationId,optional"`
	Slug *string `pulumi:"slug,optional"`
}

// GetOrganizationResult is the looked-up organization.
type GetOrganizationResult struct {
	ID                    string `pulumi:"organizationId"`
	Name                  string `pulumi:"name"`
	Slug                  string `pulumi:"slug"`
	CreatedBy             string `pulumi:"createdBy"`
	MaxAllowedMemberships int    `pulumi:"maxAllowedMemberships"`
	AdminDeleteEnabled    bool   `pulumi:"adminDeleteEnabled"`
	PublicMetadata        string `pulumi:"publicMetadata"`
	PrivateMetadata       string `pulumi:"privateMetadata" provider:"secret"`
	CreatedAt             string `pulumi:"createdAt"`
	UpdatedAt             string `pulumi:"updatedAt"`
}

func (f *GetOrganization) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk organization by ID or by slug.")
}

func (a *GetOrganizationArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The organization ID (org_...). Set either id or slug.")
	ann.Describe(&a.Slug, "The organization slug. Set either id or slug.")
}

func (f *GetOrganization) Invoke(ctx context.Context, req infer.FunctionRequest[GetOrganizationArgs]) (infer.FunctionResponse[GetOrganizationResult], error) {
	out := infer.FunctionResponse[GetOrganizationResult]{}
	id, slug := derefStr(req.Input.ID), derefStr(req.Input.Slug)
	if (id == "") == (slug == "") {
		return out, fmt.Errorf("clerk: getOrganization needs exactly one of id or slug")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	var j organizationJSON
	if err := c.do(ctx, http.MethodGet, "/organizations/"+esc(id+slug), nil, &j); err != nil {
		return out, lookupErr("organization", fmt.Sprintf("%q", id+slug), err)
	}
	st := j.state(OrganizationArgs{})
	out.Output = GetOrganizationResult{
		ID: j.ID, Name: j.Name, Slug: j.Slug, CreatedBy: j.CreatedBy,
		MaxAllowedMemberships: int(j.MaxAllowedMemberships), AdminDeleteEnabled: j.AdminDeleteEnabled,
		PublicMetadata: derefStr(st.PublicMetadata), PrivateMetadata: derefStr(st.PrivateMetadata),
		CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt,
	}
	return out, nil
}
