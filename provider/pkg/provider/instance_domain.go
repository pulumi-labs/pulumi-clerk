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

// InstanceDomain is a domain on a Clerk instance (for example a satellite
// domain), managed with the instance Backend API.
type InstanceDomain struct{}

// InstanceDomainArgs are the inputs of an InstanceDomain.
type InstanceDomainArgs struct {
	Name        string  `pulumi:"name"`
	IsSatellite *bool   `pulumi:"isSatellite,optional" provider:"replaceOnChanges"`
	ProxyURL    *string `pulumi:"proxyUrl,optional"`
}

// InstanceDomainCnameTarget is a DNS record the domain needs.
type InstanceDomainCnameTarget struct {
	Host  string `pulumi:"host"`
	Value string `pulumi:"value"`
}

// InstanceDomainState is the stored state of an InstanceDomain.
type InstanceDomainState struct {
	InstanceDomainArgs
	EffectiveIsSatellite bool                        `pulumi:"effectiveIsSatellite"`
	FrontendAPIURL       string                      `pulumi:"frontendApiUrl"`
	AccountsPortalURL    *string                     `pulumi:"accountsPortalUrl,optional"`
	DevelopmentOrigin    string                      `pulumi:"developmentOrigin"`
	CnameTargets         []InstanceDomainCnameTarget `pulumi:"cnameTargets"`
}

func (r *InstanceDomain) Annotate(a infer.Annotator) {
	a.Describe(&r, "A domain on a Clerk instance, managed through the instance Backend API (it needs the instance secret key, not the Platform API key). Whether the domain is a satellite domain is fixed at creation: changing it replaces the domain. Import with the domain ID, for example dmn_...")
}

func (a *InstanceDomainArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "The domain name, for example app.example.com.")
	ann.Describe(&a.IsSatellite, "Whether this is a satellite domain. Fixed at creation: changing this forces a new resource. Clerk defaults to false.")
	ann.Describe(&a.ProxyURL, "The proxy URL for the domain.")
}

func (t *InstanceDomainCnameTarget) Annotate(ann infer.Annotator) {
	ann.Describe(&t.Host, "DNS host name of the CNAME record.")
	ann.Describe(&t.Value, "Value the CNAME record must point to.")
}

func (s *InstanceDomainState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.EffectiveIsSatellite, "Whether Clerk treats the domain as a satellite domain, including when isSatellite is left unset.")
	ann.Describe(&s.FrontendAPIURL, "The Frontend API URL of this domain.")
	ann.Describe(&s.AccountsPortalURL, "The Accounts Portal URL of this domain.")
	ann.Describe(&s.DevelopmentOrigin, "The development origin of the domain, if any.")
	ann.Describe(&s.CnameTargets, "The DNS CNAME records the domain requires.")
}

var _ infer.CustomResource[InstanceDomainArgs, InstanceDomainState] = (*InstanceDomain)(nil)
var _ infer.CustomUpdate[InstanceDomainArgs, InstanceDomainState] = (*InstanceDomain)(nil)
var _ infer.CustomRead[InstanceDomainArgs, InstanceDomainState] = (*InstanceDomain)(nil)
var _ infer.CustomDelete[InstanceDomainState] = (*InstanceDomain)(nil)

// instanceDomainJSON is the API shape.
type instanceDomainJSON struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	IsSatellite       bool    `json:"is_satellite"`
	FrontendAPIURL    string  `json:"frontend_api_url"`
	AccountsPortalURL *string `json:"accounts_portal_url"`
	ProxyURL          *string `json:"proxy_url"`
	DevelopmentOrigin string  `json:"development_origin"`
	CnameTargets      []struct {
		Host  string `json:"host"`
		Value string `json:"value"`
	} `json:"cname_targets"`
}

// state renders the API object; args is the input part to keep (what the user
// set), and what Clerk reports for the defaulted setting goes to the output
// field.
func (j instanceDomainJSON) state(args InstanceDomainArgs) InstanceDomainState {
	targets := make([]InstanceDomainCnameTarget, 0, len(j.CnameTargets))
	for _, t := range j.CnameTargets {
		targets = append(targets, InstanceDomainCnameTarget{Host: t.Host, Value: t.Value})
	}
	return InstanceDomainState{
		InstanceDomainArgs:   args,
		EffectiveIsSatellite: j.IsSatellite,
		FrontendAPIURL:       j.FrontendAPIURL,
		AccountsPortalURL:    j.AccountsPortalURL,
		DevelopmentOrigin:    j.DevelopmentOrigin,
		CnameTargets:         targets,
	}
}

// refreshedArgs reports what the user set, with the server's value for each
// and unset optional inputs left unset; an import (no prior inputs) adopts the
// server values.
func (j instanceDomainJSON) refreshedArgs(prior InstanceDomainArgs) InstanceDomainArgs {
	sat := j.IsSatellite
	in := InstanceDomainArgs{Name: j.Name, IsSatellite: &sat, ProxyURL: j.ProxyURL}
	if prior.Name == "" {
		return in
	}
	if prior.IsSatellite == nil {
		in.IsSatellite = nil
	}
	if prior.ProxyURL == nil {
		in.ProxyURL = nil
	}
	return in
}

func findInstanceDomain(ctx context.Context, c *client, id, name string) (*instanceDomainJSON, error) {
	items, err := listAll[instanceDomainJSON](ctx, c, "/domains", nil)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if (id != "" && items[i].ID == id) || (id == "" && items[i].Name == name) {
			return &items[i], nil
		}
	}
	return nil, nil
}

func (r *InstanceDomain) Create(ctx context.Context, req infer.CreateRequest[InstanceDomainArgs]) (infer.CreateResponse[InstanceDomainState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[InstanceDomainState]{Output: InstanceDomainState{InstanceDomainArgs: in, EffectiveIsSatellite: in.IsSatellite != nil && *in.IsSatellite}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[InstanceDomainState]{}, err
	}
	sat := false
	if in.IsSatellite != nil {
		sat = *in.IsSatellite
	}
	body := map[string]any{"name": in.Name, "is_satellite": sat}
	if in.ProxyURL != nil {
		body["proxy_url"] = *in.ProxyURL
	}
	var out instanceDomainJSON
	if err := c.do(ctx, http.MethodPost, "/domains", body, &out); err != nil {
		return infer.CreateResponse[InstanceDomainState]{}, fmt.Errorf("clerk: create instance domain %q: %w", in.Name, err)
	}
	return infer.CreateResponse[InstanceDomainState]{ID: out.ID, Output: out.state(in)}, nil
}

func (r *InstanceDomain) Update(ctx context.Context, req infer.UpdateRequest[InstanceDomainArgs, InstanceDomainState]) (infer.UpdateResponse[InstanceDomainState], error) {
	if req.DryRun {
		st := req.State
		st.InstanceDomainArgs = req.Inputs
		return infer.UpdateResponse[InstanceDomainState]{Output: st}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[InstanceDomainState]{}, err
	}
	body := map[string]any{"name": req.Inputs.Name}
	if req.Inputs.ProxyURL != nil {
		body["proxy_url"] = *req.Inputs.ProxyURL
	}
	var out instanceDomainJSON
	if err := c.do(ctx, http.MethodPatch, "/domains/"+req.ID, body, &out); err != nil {
		return infer.UpdateResponse[InstanceDomainState]{}, fmt.Errorf("clerk: update instance domain %q: %w", req.ID, err)
	}
	return infer.UpdateResponse[InstanceDomainState]{Output: out.state(req.Inputs)}, nil
}

func (r *InstanceDomain) Read(ctx context.Context, req infer.ReadRequest[InstanceDomainArgs, InstanceDomainState]) (infer.ReadResponse[InstanceDomainArgs, InstanceDomainState], error) {
	empty := infer.ReadResponse[InstanceDomainArgs, InstanceDomainState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	// The API has no get-by-ID for domains, so read the list.
	got, err := findInstanceDomain(ctx, c, req.ID, "")
	if err != nil {
		return empty, fmt.Errorf("clerk: read instance domain %q: %w", req.ID, err)
	}
	if got == nil {
		return empty, nil
	}
	prior := req.Inputs
	if prior.Name == "" {
		prior = req.State.InstanceDomainArgs
	}
	args := got.refreshedArgs(prior)
	return infer.ReadResponse[InstanceDomainArgs, InstanceDomainState]{ID: got.ID, Inputs: args, State: got.state(args)}, nil
}

func (r *InstanceDomain) Delete(ctx context.Context, req infer.DeleteRequest[InstanceDomainState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/domains/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete instance domain %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetInstanceDomain looks up an instance domain by ID or name.
type GetInstanceDomain struct{}

// GetInstanceDomainArgs selects the domain; set exactly one of the fields.
type GetInstanceDomainArgs struct {
	ID   *string `pulumi:"instanceDomainId,optional"`
	Name *string `pulumi:"name,optional"`
}

// GetInstanceDomainResult is the looked-up domain.
type GetInstanceDomainResult struct {
	ID                string                      `pulumi:"instanceDomainId"`
	Name              string                      `pulumi:"name"`
	IsSatellite       bool                        `pulumi:"isSatellite"`
	ProxyURL          *string                     `pulumi:"proxyUrl,optional"`
	FrontendAPIURL    string                      `pulumi:"frontendApiUrl"`
	AccountsPortalURL *string                     `pulumi:"accountsPortalUrl,optional"`
	DevelopmentOrigin string                      `pulumi:"developmentOrigin"`
	CnameTargets      []InstanceDomainCnameTarget `pulumi:"cnameTargets"`
}

func (f *GetInstanceDomain) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a domain of the Clerk instance by ID or by name.")
}

func (a *GetInstanceDomainArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The domain ID. Set either id or name.")
	ann.Describe(&a.Name, "The domain name. Set either id or name.")
}

func (f *GetInstanceDomain) Invoke(ctx context.Context, req infer.FunctionRequest[GetInstanceDomainArgs]) (infer.FunctionResponse[GetInstanceDomainResult], error) {
	out := infer.FunctionResponse[GetInstanceDomainResult]{}
	id, name := derefStr(req.Input.ID), derefStr(req.Input.Name)
	if (id == "") == (name == "") {
		return out, fmt.Errorf("clerk: getInstanceDomain needs exactly one of id or name")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	got, err := findInstanceDomain(ctx, c, id, name)
	if err != nil {
		return out, lookupErr("instance domain", id+name, err)
	}
	if got == nil {
		return out, lookupErr("instance domain", fmt.Sprintf("%q", id+name), &apiError{Status: http.StatusNotFound})
	}
	st := got.state(InstanceDomainArgs{})
	out.Output = GetInstanceDomainResult{
		ID: got.ID, Name: got.Name, IsSatellite: got.IsSatellite, ProxyURL: got.ProxyURL,
		FrontendAPIURL: got.FrontendAPIURL, AccountsPortalURL: got.AccountsPortalURL,
		DevelopmentOrigin: got.DevelopmentOrigin, CnameTargets: st.CnameTargets,
	}
	return out, nil
}
