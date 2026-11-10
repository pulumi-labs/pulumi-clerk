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
	"net/url"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// OrganizationMembership adds a user to a Clerk organization with a role.
type OrganizationMembership struct{}

// OrganizationMembershipArgs are the inputs of an OrganizationMembership.
type OrganizationMembershipArgs struct {
	// OrganizationID is the organization to join.
	OrganizationID string `pulumi:"organizationId" provider:"replaceOnChanges"`
	// UserID is the user to add.
	UserID string `pulumi:"userId" provider:"replaceOnChanges"`
	// Role is the role key assigned to the member.
	Role string `pulumi:"role"`
}

// OrganizationMembershipState is the stored state of an OrganizationMembership.
type OrganizationMembershipState struct {
	OrganizationMembershipArgs
	MembershipID string `pulumi:"membershipId"`
	RoleName     string `pulumi:"roleName"`
	CreatedAt    string `pulumi:"createdAt"`
	UpdatedAt    string `pulumi:"updatedAt"`
}

func (r *OrganizationMembership) Annotate(a infer.Annotator) {
	a.Describe(&r, "A user's membership of a Clerk organization. The resource ID is <organizationId>/<userId>; import with that composite ID (for example org_123/user_456). The organization and user cannot be changed: changing either replaces the membership. Only the role is updated in place.")
}

func (a *OrganizationMembershipArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.OrganizationID, "ID of the organization. Changing this forces a new resource.")
	ann.Describe(&a.UserID, "ID of the user to add as a member. Changing this forces a new resource.")
	ann.Describe(&a.Role, "Role key to assign, for example org:admin or org:member.")
}

func (s *OrganizationMembershipState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.MembershipID, "Clerk's own ID for the membership (orgmem_...). The Pulumi resource ID is <organizationId>/<userId>.")
	ann.Describe(&s.RoleName, "Display name of the assigned role.")
	ann.Describe(&s.CreatedAt, "When the membership was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the membership was last updated (RFC 3339).")
}

var _ infer.CustomResource[OrganizationMembershipArgs, OrganizationMembershipState] = (*OrganizationMembership)(nil)
var _ infer.CustomRead[OrganizationMembershipArgs, OrganizationMembershipState] = (*OrganizationMembership)(nil)
var _ infer.CustomUpdate[OrganizationMembershipArgs, OrganizationMembershipState] = (*OrganizationMembership)(nil)
var _ infer.CustomDelete[OrganizationMembershipState] = (*OrganizationMembership)(nil)

// organizationMembershipJSON is the API shape.
type organizationMembershipJSON struct {
	ID             string `json:"id"`
	Role           string `json:"role"`
	RoleName       string `json:"role_name"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	PublicUserData *struct {
		UserID string `json:"user_id"`
	} `json:"public_user_data"`
}

func (j organizationMembershipJSON) state(orgID, userID string) OrganizationMembershipState {
	return OrganizationMembershipState{
		OrganizationMembershipArgs: OrganizationMembershipArgs{OrganizationID: orgID, UserID: userID, Role: j.Role},
		MembershipID:               j.ID,
		RoleName:                   j.RoleName,
		CreatedAt:                  millisToRFC3339(j.CreatedAt),
		UpdatedAt:                  millisToRFC3339(j.UpdatedAt),
	}
}

func membershipID(orgID, userID string) string { return orgID + "/" + userID }

// findOrganizationMembership lists the user's memberships of the organization
// (there is no get-by-user endpoint); a missing organization returns nil.
func findOrganizationMembership(ctx context.Context, c *client, orgID, userID string) (*organizationMembershipJSON, error) {
	items, err := listAll[organizationMembershipJSON](ctx, c, "/organizations/"+esc(orgID)+"/memberships", url.Values{"user_id": {userID}})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	for i := range items {
		if items[i].PublicUserData != nil && items[i].PublicUserData.UserID == userID {
			return &items[i], nil
		}
	}
	return nil, nil
}

func (r *OrganizationMembership) Create(ctx context.Context, req infer.CreateRequest[OrganizationMembershipArgs]) (infer.CreateResponse[OrganizationMembershipState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[OrganizationMembershipState]{Output: OrganizationMembershipState{OrganizationMembershipArgs: in}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[OrganizationMembershipState]{}, err
	}
	var out organizationMembershipJSON
	body := map[string]any{"user_id": in.UserID, "role": in.Role}
	if err := c.do(ctx, http.MethodPost, "/organizations/"+esc(in.OrganizationID)+"/memberships", body, &out); err != nil {
		return infer.CreateResponse[OrganizationMembershipState]{}, fmt.Errorf("clerk: add user %q to organization %q: %w", in.UserID, in.OrganizationID, err)
	}
	return infer.CreateResponse[OrganizationMembershipState]{ID: membershipID(in.OrganizationID, in.UserID), Output: out.state(in.OrganizationID, in.UserID)}, nil
}

func (r *OrganizationMembership) Read(ctx context.Context, req infer.ReadRequest[OrganizationMembershipArgs, OrganizationMembershipState]) (infer.ReadResponse[OrganizationMembershipArgs, OrganizationMembershipState], error) {
	empty := infer.ReadResponse[OrganizationMembershipArgs, OrganizationMembershipState]{}
	orgID, userID, err := splitID("organization membership", "organizationId", req.ID)
	if err != nil {
		return empty, err
	}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := findOrganizationMembership(ctx, c, orgID, userID)
	if err != nil {
		return empty, fmt.Errorf("clerk: read organization membership %q: %w", req.ID, err)
	}
	if got == nil {
		if err := notFoundOnImport("organization membership", req.ID, req.Inputs, req.State); err != nil {
			return empty, err
		}
		return empty, nil
	}
	st := got.state(orgID, userID)
	return infer.ReadResponse[OrganizationMembershipArgs, OrganizationMembershipState]{ID: req.ID, Inputs: st.OrganizationMembershipArgs, State: st}, nil
}

func (r *OrganizationMembership) Update(ctx context.Context, req infer.UpdateRequest[OrganizationMembershipArgs, OrganizationMembershipState]) (infer.UpdateResponse[OrganizationMembershipState], error) {
	if req.DryRun {
		st := req.State
		st.Role = req.Inputs.Role
		return infer.UpdateResponse[OrganizationMembershipState]{Output: st}, nil
	}
	orgID, userID, err := splitID("organization membership", "organizationId", req.ID)
	if err != nil {
		return infer.UpdateResponse[OrganizationMembershipState]{}, err
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[OrganizationMembershipState]{}, err
	}
	var out organizationMembershipJSON
	if err := c.do(ctx, http.MethodPatch, "/organizations/"+esc(orgID)+"/memberships/"+esc(userID), map[string]any{"role": req.Inputs.Role}, &out); err != nil {
		return infer.UpdateResponse[OrganizationMembershipState]{}, fmt.Errorf("clerk: update organization membership %q: %w", req.ID, err)
	}
	return infer.UpdateResponse[OrganizationMembershipState]{Output: out.state(orgID, userID)}, nil
}

func (r *OrganizationMembership) Delete(ctx context.Context, req infer.DeleteRequest[OrganizationMembershipState]) (infer.DeleteResponse, error) {
	orgID, userID, err := splitID("organization membership", "organizationId", req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/organizations/"+esc(orgID)+"/memberships/"+esc(userID), nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete organization membership %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetOrganizationMembership looks up a user's membership of an organization.
type GetOrganizationMembership struct{}

// GetOrganizationMembershipArgs selects the membership.
type GetOrganizationMembershipArgs struct {
	OrganizationID string `pulumi:"organizationId"`
	UserID         string `pulumi:"userId"`
}

// GetOrganizationMembershipResult is the looked-up membership.
type GetOrganizationMembershipResult struct {
	ID             string `pulumi:"organizationMembershipId"`
	MembershipID   string `pulumi:"membershipId"`
	OrganizationID string `pulumi:"organizationId"`
	UserID         string `pulumi:"userId"`
	Role           string `pulumi:"role"`
	RoleName       string `pulumi:"roleName"`
	CreatedAt      string `pulumi:"createdAt"`
	UpdatedAt      string `pulumi:"updatedAt"`
}

func (f *GetOrganizationMembership) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a user's membership of a Clerk organization. The id is <organizationId>/<userId>.")
}

func (a *GetOrganizationMembershipArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.OrganizationID, "ID of the organization.")
	ann.Describe(&a.UserID, "ID of the member user.")
}

func (f *GetOrganizationMembership) Invoke(ctx context.Context, req infer.FunctionRequest[GetOrganizationMembershipArgs]) (infer.FunctionResponse[GetOrganizationMembershipResult], error) {
	out := infer.FunctionResponse[GetOrganizationMembershipResult]{}
	in := req.Input
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	what := fmt.Sprintf("for user %q in organization %q", in.UserID, in.OrganizationID)
	got, err := findOrganizationMembership(ctx, c, in.OrganizationID, in.UserID)
	if err != nil {
		return out, lookupErr("organization membership", what, err)
	}
	if got == nil {
		return out, lookupErr("organization membership", what, &apiError{Status: http.StatusNotFound})
	}
	st := got.state(in.OrganizationID, in.UserID)
	out.Output = GetOrganizationMembershipResult{
		ID: membershipID(in.OrganizationID, in.UserID), MembershipID: got.ID, OrganizationID: in.OrganizationID, UserID: in.UserID,
		Role: got.Role, RoleName: got.RoleName, CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt,
	}
	return out, nil
}
