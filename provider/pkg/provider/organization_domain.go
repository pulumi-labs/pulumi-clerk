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

// OrganizationDomain is a verified or pending domain of a Clerk organization,
// used for automatic invitations and suggestions.
type OrganizationDomain struct{}

// OrganizationDomainArgs are the inputs of an OrganizationDomain.
type OrganizationDomainArgs struct {
	// OrganizationID is the organization the domain belongs to.
	OrganizationID string `pulumi:"organizationId" provider:"replaceOnChanges"`
	// Name is the domain name, for example example.com.
	Name string `pulumi:"name" provider:"replaceOnChanges"`
	// EnrollmentMode controls how users with this email domain join.
	EnrollmentMode *string `pulumi:"enrollmentMode,optional"`
	// Verified marks the domain verified without sending a verification email.
	Verified *bool `pulumi:"verified,optional"`
}

// OrganizationDomainState is the stored state of an OrganizationDomain.
type OrganizationDomainState struct {
	OrganizationDomainArgs
	AffiliationEmailAddress string `pulumi:"affiliationEmailAddress"`
	VerificationStatus      string `pulumi:"verificationStatus"`
	CreatedAt               string `pulumi:"createdAt"`
	UpdatedAt               string `pulumi:"updatedAt"`
}

func (r *OrganizationDomain) Annotate(a infer.Annotator) {
	a.Describe(&r, "A domain attached to a Clerk organization. The resource ID is <organizationId>/<domainId>; import with that composite ID (for example org_123/orgdmn_456). The organization and the name cannot be changed: changing either replaces the domain.")
}

func (a *OrganizationDomainArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.OrganizationID, "ID of the organization. Changing this forces a new resource.")
	ann.Describe(&a.Name, "The domain name, for example example.com. Changing this forces a new resource.")
	ann.Describe(&a.EnrollmentMode, "Enrollment mode: manual_invitation, automatic_invitation or automatic_suggestion. Defaults to the instance setting when unset.")
	ann.Describe(&a.Verified, "Whether the domain is verified. Set true to verify the domain on creation without an email check. Reports the domain's current verification state.")
}

func (s *OrganizationDomainState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.AffiliationEmailAddress, "The email address used for domain affiliation, when verification is by email.")
	ann.Describe(&s.VerificationStatus, "The domain's verification status, for example unverified or verified.")
	ann.Describe(&s.CreatedAt, "When the domain was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the domain was last updated (RFC 3339).")
}

var _ infer.CustomResource[OrganizationDomainArgs, OrganizationDomainState] = (*OrganizationDomain)(nil)
var _ infer.CustomRead[OrganizationDomainArgs, OrganizationDomainState] = (*OrganizationDomain)(nil)
var _ infer.CustomUpdate[OrganizationDomainArgs, OrganizationDomainState] = (*OrganizationDomain)(nil)
var _ infer.CustomDiff[OrganizationDomainArgs, OrganizationDomainState] = (*OrganizationDomain)(nil)
var _ infer.CustomDelete[OrganizationDomainState] = (*OrganizationDomain)(nil)

// organizationDomainJSON is the API shape.
type organizationDomainJSON struct {
	ID                      string  `json:"id"`
	OrganizationID          string  `json:"organization_id"`
	Name                    string  `json:"name"`
	EnrollmentMode          string  `json:"enrollment_mode"`
	AffiliationEmailAddress *string `json:"affiliation_email_address"`
	Verification            *struct {
		Status string `json:"status"`
	} `json:"verification"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

func (j organizationDomainJSON) state(orgID string, prev OrganizationDomainArgs) OrganizationDomainState {
	mode := j.EnrollmentMode
	st := OrganizationDomainState{
		OrganizationDomainArgs:  OrganizationDomainArgs{OrganizationID: orgID, Name: j.Name, Verified: prev.Verified},
		AffiliationEmailAddress: derefStr(j.AffiliationEmailAddress),
		CreatedAt:               millisToRFC3339(j.CreatedAt),
		UpdatedAt:               millisToRFC3339(j.UpdatedAt),
	}
	if mode != "" {
		st.EnrollmentMode = &mode
	}
	if j.Verification != nil {
		st.VerificationStatus = j.Verification.Status
		v := j.Verification.Status == "verified"
		st.Verified = &v
	}
	return st
}

func organizationDomainInputs(prior OrganizationDomainArgs, st OrganizationDomainState, imported bool) OrganizationDomainArgs {
	in := OrganizationDomainArgs{OrganizationID: st.OrganizationID, Name: st.Name}
	if imported || prior.EnrollmentMode != nil {
		in.EnrollmentMode = st.EnrollmentMode
	}
	if imported || prior.Verified != nil {
		in.Verified = st.Verified
	}
	return in
}

func domainID(orgID, id string) string { return orgID + "/" + id }

// Diff replaces on organization or name changes and treats unset optional
// inputs as "keep what Clerk chose".
func (r *OrganizationDomain) Diff(_ context.Context, req infer.DiffRequest[OrganizationDomainArgs, OrganizationDomainState]) (infer.DiffResponse, error) {
	in, old := req.Inputs, req.State
	var d diffAcc
	if in.OrganizationID != old.OrganizationID {
		d.replace("organizationId")
	}
	if in.Name != old.Name {
		d.replace("name")
	}
	if in.EnrollmentMode != nil && derefStr(in.EnrollmentMode) != derefStr(old.EnrollmentMode) {
		d.update("enrollmentMode")
	}
	if in.Verified != nil && (old.Verified == nil || *in.Verified != *old.Verified) {
		d.update("verified")
	}
	return d.resp, nil
}

func (r *OrganizationDomain) Create(ctx context.Context, req infer.CreateRequest[OrganizationDomainArgs]) (infer.CreateResponse[OrganizationDomainState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[OrganizationDomainState]{Output: OrganizationDomainState{OrganizationDomainArgs: in}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[OrganizationDomainState]{}, err
	}
	body := map[string]any{"name": in.Name}
	if in.EnrollmentMode != nil {
		body["enrollment_mode"] = *in.EnrollmentMode
	}
	if in.Verified != nil {
		body["verified"] = *in.Verified
	}
	var out organizationDomainJSON
	if err := c.do(ctx, http.MethodPost, "/organizations/"+esc(in.OrganizationID)+"/domains", body, &out); err != nil {
		return infer.CreateResponse[OrganizationDomainState]{}, fmt.Errorf("clerk: create domain %q in organization %q: %w", in.Name, in.OrganizationID, err)
	}
	return infer.CreateResponse[OrganizationDomainState]{ID: domainID(in.OrganizationID, out.ID), Output: out.state(in.OrganizationID, in)}, nil
}

// findOrganizationDomain reads the organization's domain list (there is no
// get-by-ID); a missing organization or domain returns nil.
func findOrganizationDomain(ctx context.Context, c *client, orgID, id string) (*organizationDomainJSON, error) {
	items, err := listAll[organizationDomainJSON](ctx, c, "/organizations/"+esc(orgID)+"/domains", nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	return nil, nil
}

func (r *OrganizationDomain) Read(ctx context.Context, req infer.ReadRequest[OrganizationDomainArgs, OrganizationDomainState]) (infer.ReadResponse[OrganizationDomainArgs, OrganizationDomainState], error) {
	empty := infer.ReadResponse[OrganizationDomainArgs, OrganizationDomainState]{}
	orgID, id, err := splitID("organization domain", "organizationId", req.ID)
	if err != nil {
		return empty, err
	}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := findOrganizationDomain(ctx, c, orgID, id)
	if err != nil {
		return empty, fmt.Errorf("clerk: read organization domain %q: %w", req.ID, err)
	}
	if got == nil {
		return empty, nil
	}
	imported := req.Inputs.Name == "" && req.State.Name == ""
	st := got.state(orgID, req.State.OrganizationDomainArgs)
	return infer.ReadResponse[OrganizationDomainArgs, OrganizationDomainState]{
		ID: domainID(orgID, got.ID), Inputs: organizationDomainInputs(req.Inputs, st, imported), State: st,
	}, nil
}

func (r *OrganizationDomain) Update(ctx context.Context, req infer.UpdateRequest[OrganizationDomainArgs, OrganizationDomainState]) (infer.UpdateResponse[OrganizationDomainState], error) {
	in, old := req.Inputs, req.State
	if req.DryRun {
		st := old
		if in.EnrollmentMode != nil {
			st.EnrollmentMode = in.EnrollmentMode
		}
		if in.Verified != nil {
			st.Verified = in.Verified
		}
		return infer.UpdateResponse[OrganizationDomainState]{Output: st}, nil
	}
	orgID, id, err := splitID("organization domain", "organizationId", req.ID)
	if err != nil {
		return infer.UpdateResponse[OrganizationDomainState]{}, err
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[OrganizationDomainState]{}, err
	}
	body := map[string]any{}
	if in.EnrollmentMode != nil {
		body["enrollment_mode"] = *in.EnrollmentMode
	}
	if in.Verified != nil && (old.Verified == nil || *in.Verified != *old.Verified) {
		body["verified"] = *in.Verified
	}
	var out organizationDomainJSON
	if err := c.do(ctx, http.MethodPatch, "/organizations/"+esc(orgID)+"/domains/"+esc(id), body, &out); err != nil {
		return infer.UpdateResponse[OrganizationDomainState]{}, fmt.Errorf("clerk: update organization domain %q: %w", req.ID, err)
	}
	return infer.UpdateResponse[OrganizationDomainState]{Output: out.state(orgID, in)}, nil
}

func (r *OrganizationDomain) Delete(ctx context.Context, req infer.DeleteRequest[OrganizationDomainState]) (infer.DeleteResponse, error) {
	orgID, id, err := splitID("organization domain", "organizationId", req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/organizations/"+esc(orgID)+"/domains/"+esc(id), nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete organization domain %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetOrganizationDomain looks up a domain of an organization by ID or name.
type GetOrganizationDomain struct{}

// GetOrganizationDomainArgs selects the domain; set organizationId and exactly one of id or name.
type GetOrganizationDomainArgs struct {
	OrganizationID string  `pulumi:"organizationId"`
	ID             *string `pulumi:"organizationDomainId,optional"`
	Name           *string `pulumi:"name,optional"`
}

// GetOrganizationDomainResult is the looked-up domain.
type GetOrganizationDomainResult struct {
	ID                      string `pulumi:"organizationDomainId"`
	OrganizationID          string `pulumi:"organizationId"`
	Name                    string `pulumi:"name"`
	EnrollmentMode          string `pulumi:"enrollmentMode"`
	Verified                bool   `pulumi:"verified"`
	AffiliationEmailAddress string `pulumi:"affiliationEmailAddress"`
	VerificationStatus      string `pulumi:"verificationStatus"`
	CreatedAt               string `pulumi:"createdAt"`
	UpdatedAt               string `pulumi:"updatedAt"`
}

func (f *GetOrganizationDomain) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a domain of a Clerk organization by domain ID or by name.")
}

func (a *GetOrganizationDomainArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.OrganizationID, "ID of the organization that owns the domain.")
	ann.Describe(&a.ID, "The domain ID. Set either id or name.")
	ann.Describe(&a.Name, "The domain name. Set either id or name.")
}

func (f *GetOrganizationDomain) Invoke(ctx context.Context, req infer.FunctionRequest[GetOrganizationDomainArgs]) (infer.FunctionResponse[GetOrganizationDomainResult], error) {
	out := infer.FunctionResponse[GetOrganizationDomainResult]{}
	id, name := derefStr(req.Input.ID), derefStr(req.Input.Name)
	if (id == "") == (name == "") {
		return out, fmt.Errorf("clerk: getOrganizationDomain needs exactly one of id or name")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	items, err := listAll[organizationDomainJSON](ctx, c, "/organizations/"+esc(req.Input.OrganizationID)+"/domains", nil)
	if err != nil {
		return out, lookupErr("organization domain", id+name, err)
	}
	for _, it := range items {
		if (id != "" && it.ID == id) || (id == "" && it.Name == name) {
			st := it.state(req.Input.OrganizationID, OrganizationDomainArgs{})
			out.Output = GetOrganizationDomainResult{
				ID: it.ID, OrganizationID: req.Input.OrganizationID, Name: it.Name, EnrollmentMode: it.EnrollmentMode,
				Verified: st.Verified != nil && *st.Verified, AffiliationEmailAddress: st.AffiliationEmailAddress,
				VerificationStatus: st.VerificationStatus, CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt,
			}
			return out, nil
		}
	}
	return out, lookupErr("organization domain", fmt.Sprintf("%q", id+name), &apiError{Status: http.StatusNotFound})
}
