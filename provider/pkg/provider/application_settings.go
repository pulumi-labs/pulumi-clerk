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
	"errors"
	"fmt"
	"net/http"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// applicationSettingsID is the fixed ID of the instance's single settings object.
const applicationSettingsID = "application_settings"

// ApplicationSettings are the organization feature settings of a Clerk
// instance. There is exactly one per instance.
type ApplicationSettings struct{}

// ApplicationSettingsArgs are the inputs of ApplicationSettings.
type ApplicationSettingsArgs struct {
	EnableOrganizations   bool  `pulumi:"enableOrganizations"`
	MaxAllowedMemberships *int  `pulumi:"maxAllowedMemberships,optional"`
	AdminDeleteEnabled    *bool `pulumi:"adminDeleteEnabled,optional"`
	DomainsEnabled        *bool `pulumi:"domainsEnabled,optional"`
}

// ApplicationSettingsState is the stored state of ApplicationSettings.
type ApplicationSettingsState struct {
	ApplicationSettingsArgs
	CurrentMaxAllowedMemberships int    `pulumi:"currentMaxAllowedMemberships"`
	CurrentAdminDeleteEnabled    bool   `pulumi:"currentAdminDeleteEnabled"`
	CurrentDomainsEnabled        bool   `pulumi:"currentDomainsEnabled"`
	MaxAllowedRoles              int    `pulumi:"maxAllowedRoles"`
	MaxAllowedPermissions        int    `pulumi:"maxAllowedPermissions"`
	CreatorRole                  string `pulumi:"creatorRole"`
	DomainsDefaultRole           string `pulumi:"domainsDefaultRole"`
}

func (r *ApplicationSettings) Annotate(a infer.Annotator) {
	a.Describe(&r, "The organization feature settings of a Clerk instance (the instance behind the clerk:apiKey). This is a singleton: only one can exist per instance, and its ID is always application_settings. The Backend API has no read endpoint for these settings, so refresh and import read the current values by sending an empty update. Optional inputs left unset are not managed: Clerk keeps its current value, which is shown in the matching current* output. Deleting the resource only stops managing the settings: the instance keeps its current values, including whether organizations are enabled, because disabling Organizations would affect every user of the instance and Clerk keeps no prior value to restore. Import with the ID application_settings.")
}

func (a *ApplicationSettingsArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.EnableOrganizations, "Whether organizations are enabled for this instance.")
	ann.Describe(&a.MaxAllowedMemberships, "Default maximum number of memberships per organization. Leave unset to keep Clerk's current value; see currentMaxAllowedMemberships.")
	ann.Describe(&a.AdminDeleteEnabled, "Whether organization admins can delete organizations. Leave unset to keep Clerk's current value; see currentAdminDeleteEnabled.")
	ann.Describe(&a.DomainsEnabled, "Whether domain-based organization enrollment is enabled. Leave unset to keep Clerk's current value; see currentDomainsEnabled.")
}

func (s *ApplicationSettingsState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.CurrentMaxAllowedMemberships, "The default maximum number of memberships per organization Clerk currently applies.")
	ann.Describe(&s.CurrentAdminDeleteEnabled, "Whether organization admins can currently delete organizations.")
	ann.Describe(&s.CurrentDomainsEnabled, "Whether domain-based organization enrollment is currently enabled.")
	ann.Describe(&s.MaxAllowedRoles, "Maximum number of roles allowed per organization.")
	ann.Describe(&s.MaxAllowedPermissions, "Maximum number of permissions allowed per organization.")
	ann.Describe(&s.CreatorRole, "The role assigned to the creator of a new organization.")
	ann.Describe(&s.DomainsDefaultRole, "The default role of users added through domain enrollment.")
}

var _ infer.CustomResource[ApplicationSettingsArgs, ApplicationSettingsState] = (*ApplicationSettings)(nil)
var _ infer.CustomRead[ApplicationSettingsArgs, ApplicationSettingsState] = (*ApplicationSettings)(nil)
var _ infer.CustomUpdate[ApplicationSettingsArgs, ApplicationSettingsState] = (*ApplicationSettings)(nil)
var _ infer.CustomDelete[ApplicationSettingsState] = (*ApplicationSettings)(nil)

// applicationSettingsJSON is the API shape.
type applicationSettingsJSON struct {
	Enabled               bool   `json:"enabled"`
	MaxAllowedMemberships int    `json:"max_allowed_memberships"`
	MaxAllowedRoles       int    `json:"max_allowed_roles"`
	MaxAllowedPermissions int    `json:"max_allowed_permissions"`
	CreatorRole           string `json:"creator_role"`
	AdminDeleteEnabled    bool   `json:"admin_delete_enabled"`
	DomainsEnabled        bool   `json:"domains_enabled"`
	DomainsDefaultRole    string `json:"domains_default_role"`
}

// state keeps the user's optional inputs as given and reports the server's
// values in the current* outputs.
func (j applicationSettingsJSON) state(args ApplicationSettingsArgs) ApplicationSettingsState {
	args.EnableOrganizations = j.Enabled
	return ApplicationSettingsState{
		ApplicationSettingsArgs:      args,
		CurrentMaxAllowedMemberships: j.MaxAllowedMemberships,
		CurrentAdminDeleteEnabled:    j.AdminDeleteEnabled,
		CurrentDomainsEnabled:        j.DomainsEnabled,
		MaxAllowedRoles:              j.MaxAllowedRoles,
		MaxAllowedPermissions:        j.MaxAllowedPermissions,
		CreatorRole:                  j.CreatorRole,
		DomainsDefaultRole:           j.DomainsDefaultRole,
	}
}

func patchApplicationSettings(ctx context.Context, c *client, body map[string]any) (*applicationSettingsJSON, error) {
	var out applicationSettingsJSON
	if err := c.do(ctx, http.MethodPatch, "/instance/organization_settings", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func settingsBody(a ApplicationSettingsArgs) map[string]any {
	body := map[string]any{"enabled": a.EnableOrganizations}
	if a.MaxAllowedMemberships != nil {
		body["max_allowed_memberships"] = *a.MaxAllowedMemberships
	}
	if a.AdminDeleteEnabled != nil {
		body["admin_delete_enabled"] = *a.AdminDeleteEnabled
	}
	if a.DomainsEnabled != nil {
		body["domains_enabled"] = *a.DomainsEnabled
	}
	return body
}

func (r *ApplicationSettings) apply(ctx context.Context, args ApplicationSettingsArgs, dry bool) (ApplicationSettingsState, error) {
	if dry {
		return ApplicationSettingsState{ApplicationSettingsArgs: args}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return ApplicationSettingsState{}, err
	}
	got, err := patchApplicationSettings(ctx, c, settingsBody(args))
	if err != nil {
		return ApplicationSettingsState{}, fmt.Errorf("clerk: update application settings: %w", err)
	}
	return got.state(args), nil
}

func (r *ApplicationSettings) Create(ctx context.Context, req infer.CreateRequest[ApplicationSettingsArgs]) (infer.CreateResponse[ApplicationSettingsState], error) {
	st, err := r.apply(ctx, req.Inputs, req.DryRun)
	if err != nil {
		return infer.CreateResponse[ApplicationSettingsState]{}, err
	}
	return infer.CreateResponse[ApplicationSettingsState]{ID: applicationSettingsID, Output: st}, nil
}

func (r *ApplicationSettings) Update(ctx context.Context, req infer.UpdateRequest[ApplicationSettingsArgs, ApplicationSettingsState]) (infer.UpdateResponse[ApplicationSettingsState], error) {
	st, err := r.apply(ctx, req.Inputs, req.DryRun)
	if err != nil {
		return infer.UpdateResponse[ApplicationSettingsState]{}, err
	}
	return infer.UpdateResponse[ApplicationSettingsState]{Output: st}, nil
}

func (r *ApplicationSettings) Read(ctx context.Context, req infer.ReadRequest[ApplicationSettingsArgs, ApplicationSettingsState]) (infer.ReadResponse[ApplicationSettingsArgs, ApplicationSettingsState], error) {
	empty := infer.ReadResponse[ApplicationSettingsArgs, ApplicationSettingsState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	// No GET endpoint exists, so an empty PATCH (which changes nothing)
	// returns the current settings.
	got, err := patchApplicationSettings(ctx, c, map[string]any{})
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) && (ae.Status == http.StatusBadRequest || ae.Status == http.StatusUnprocessableEntity) && req.State.CreatorRole != "" {
			// The API refused an empty update: keep the last known state.
			return infer.ReadResponse[ApplicationSettingsArgs, ApplicationSettingsState]{ID: req.ID, Inputs: req.State.ApplicationSettingsArgs, State: req.State}, nil
		}
		return empty, fmt.Errorf("clerk: read application settings: %w", err)
	}
	args := req.Inputs
	if req.State.CreatorRole == "" && req.Inputs.MaxAllowedMemberships == nil && req.Inputs.AdminDeleteEnabled == nil && req.Inputs.DomainsEnabled == nil {
		// Import: every input is optional and Clerk fills in defaults, so adopt
		// none of them (#1563); the state outputs carry the server values.
		args = ApplicationSettingsArgs{}
	} else {
		// Refresh: managed optional inputs follow the server so drift shows;
		// unmanaged ones stay unset.
		if args.MaxAllowedMemberships != nil {
			args.MaxAllowedMemberships = &got.MaxAllowedMemberships
		}
		if args.AdminDeleteEnabled != nil {
			args.AdminDeleteEnabled = &got.AdminDeleteEnabled
		}
		if args.DomainsEnabled != nil {
			args.DomainsEnabled = &got.DomainsEnabled
		}
	}
	st := got.state(args)
	return infer.ReadResponse[ApplicationSettingsArgs, ApplicationSettingsState]{ID: applicationSettingsID, Inputs: st.ApplicationSettingsArgs, State: st}, nil
}

// Delete only removes the resource from state. The settings are a singleton the instance always has, and
// turning Organizations off would affect every user of a shared instance with no way to restore the prior
// value (Clerk keeps none), so the instance is left exactly as it is (#1683). The Terraform provider disabled
// Organizations here; this provider deliberately does not.
func (r *ApplicationSettings) Delete(_ context.Context, _ infer.DeleteRequest[ApplicationSettingsState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

// GetApplicationSettings reads the organization settings of the instance.
type GetApplicationSettings struct{}

// GetApplicationSettingsArgs has no inputs.
type GetApplicationSettingsArgs struct{}

// GetApplicationSettingsResult is the current organization settings.
type GetApplicationSettingsResult struct {
	EnableOrganizations   bool   `pulumi:"enableOrganizations"`
	MaxAllowedMemberships int    `pulumi:"maxAllowedMemberships"`
	MaxAllowedRoles       int    `pulumi:"maxAllowedRoles"`
	MaxAllowedPermissions int    `pulumi:"maxAllowedPermissions"`
	CreatorRole           string `pulumi:"creatorRole"`
	AdminDeleteEnabled    bool   `pulumi:"adminDeleteEnabled"`
	DomainsEnabled        bool   `pulumi:"domainsEnabled"`
	DomainsDefaultRole    string `pulumi:"domainsDefaultRole"`
}

func (f *GetApplicationSettings) Annotate(a infer.Annotator) {
	a.Describe(&f, "Read the organization settings of the Clerk instance behind clerk:apiKey. The Backend API has no read endpoint, so this sends an empty update, which changes nothing.")
}

func (r *GetApplicationSettingsResult) Annotate(ann infer.Annotator) {
	ann.Describe(&r.EnableOrganizations, "Whether organizations are enabled.")
	ann.Describe(&r.MaxAllowedMemberships, "Default maximum number of memberships per organization.")
	ann.Describe(&r.MaxAllowedRoles, "Maximum number of roles per organization.")
	ann.Describe(&r.MaxAllowedPermissions, "Maximum number of permissions per organization.")
	ann.Describe(&r.CreatorRole, "The role assigned to the creator of a new organization.")
	ann.Describe(&r.AdminDeleteEnabled, "Whether organization admins can delete organizations.")
	ann.Describe(&r.DomainsEnabled, "Whether domain-based organization enrollment is enabled.")
	ann.Describe(&r.DomainsDefaultRole, "The default role of users added through domain enrollment.")
}

func (f *GetApplicationSettings) Invoke(ctx context.Context, req infer.FunctionRequest[GetApplicationSettingsArgs]) (infer.FunctionResponse[GetApplicationSettingsResult], error) {
	out := infer.FunctionResponse[GetApplicationSettingsResult]{}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	got, err := patchApplicationSettings(ctx, c, map[string]any{})
	if err != nil {
		return out, lookupErr("application settings", "of the instance", err)
	}
	out.Output = GetApplicationSettingsResult{EnableOrganizations: got.Enabled, MaxAllowedMemberships: got.MaxAllowedMemberships,
		MaxAllowedRoles: got.MaxAllowedRoles, MaxAllowedPermissions: got.MaxAllowedPermissions, CreatorRole: got.CreatorRole,
		AdminDeleteEnabled: got.AdminDeleteEnabled, DomainsEnabled: got.DomainsEnabled, DomainsDefaultRole: got.DomainsDefaultRole}
	return out, nil
}
