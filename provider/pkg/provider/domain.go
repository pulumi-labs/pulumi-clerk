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
	"strings"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Domain is a custom domain on a Clerk application, managed through the
// Platform API (beta).
type Domain struct{}

// DomainCNAMETarget is a DNS record the domain needs.
type DomainCNAMETarget struct {
	Host     string `pulumi:"host"`
	Value    string `pulumi:"value"`
	Required bool   `pulumi:"required"`
}

func (t *DomainCNAMETarget) Annotate(a infer.Annotator) {
	a.Describe(&t.Host, "The CNAME host.")
	a.Describe(&t.Value, "The CNAME value.")
	a.Describe(&t.Required, "Whether this CNAME record is required.")
}

// DomainArgs are the inputs of a Domain.
type DomainArgs struct {
	ApplicationID string  `pulumi:"applicationId" provider:"replaceOnChanges"`
	Name          string  `pulumi:"name" provider:"replaceOnChanges"`
	ProxyPath     *string `pulumi:"proxyPath,optional" provider:"replaceOnChanges"`
}

// DomainState is the stored state of a Domain.
type DomainState struct {
	DomainArgs
	IsSatellite      bool                `pulumi:"isSatellite"`
	IsProviderDomain bool                `pulumi:"isProviderDomain"`
	FrontendAPIURL   string              `pulumi:"frontendApiUrl"`
	CNAMETargets     []DomainCNAMETarget `pulumi:"cnameTargets"`
}

func (r *Domain) Annotate(a infer.Annotator) {
	a.Describe(&r, "A domain on a Clerk application, managed through the Clerk Platform API. The Platform API is in beta and must be enabled by Clerk for your account; configure it with the clerk:platformApiKey provider setting. Domains cannot be updated: changing any input replaces the domain. The resource ID is <applicationId>/<domainId>; import with that ID (for example: pulumi import clerk:index/domain:Domain d app_123/dmn_456). The API does not return proxyPath, so it is kept as configured.")
}

func (a *DomainArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ApplicationID, "The ID of the application this domain belongs to. Changing this forces a new resource.")
	ann.Describe(&a.Name, "The domain name, for example app.example.com. Changing this forces a new resource.")
	ann.Describe(&a.ProxyPath, "Proxy path for the domain. Create-only: changing this forces a new resource.")
}

func (s *DomainState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.IsSatellite, "Whether this is a satellite domain.")
	ann.Describe(&s.IsProviderDomain, "Whether this is a provider domain.")
	ann.Describe(&s.FrontendAPIURL, "The frontend API URL of this domain.")
	ann.Describe(&s.CNAMETargets, "The CNAME records to create in DNS for this domain.")
}

var _ infer.CustomResource[DomainArgs, DomainState] = (*Domain)(nil)
var _ infer.CustomRead[DomainArgs, DomainState] = (*Domain)(nil)
var _ infer.CustomDelete[DomainState] = (*Domain)(nil)

// domainJSON is the Platform API shape.
type domainJSON struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	IsSatellite      bool   `json:"is_satellite"`
	IsProviderDomain bool   `json:"is_provider_domain"`
	FrontendAPIURL   string `json:"frontend_api_url"`
	CNAMETargets     []struct {
		Host     string `json:"host"`
		Value    string `json:"value"`
		Required bool   `json:"required"`
	} `json:"cname_targets"`
}

func (j domainJSON) state(args DomainArgs) DomainState {
	targets := make([]DomainCNAMETarget, 0, len(j.CNAMETargets))
	for _, t := range j.CNAMETargets {
		targets = append(targets, DomainCNAMETarget{Host: t.Host, Value: t.Value, Required: t.Required})
	}
	args.Name = j.Name
	return DomainState{DomainArgs: args, IsSatellite: j.IsSatellite, IsProviderDomain: j.IsProviderDomain, FrontendAPIURL: j.FrontendAPIURL, CNAMETargets: targets}
}

// splitDomainID splits <applicationId>/<domainId>.
func splitDomainID(id string) (appID, domainID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("clerk: invalid domain ID %q: expected <applicationId>/<domainId>", id)
	}
	return parts[0], parts[1], nil
}

func getDomainJSON(ctx context.Context, c *client, appID, domainID string) (*domainJSON, error) {
	var out domainJSON
	if err := c.do(ctx, http.MethodGet, "/platform/applications/"+appID+"/domains/"+domainID, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Domain) Create(ctx context.Context, req infer.CreateRequest[DomainArgs]) (infer.CreateResponse[DomainState], error) {
	if req.DryRun {
		return infer.CreateResponse[DomainState]{Output: DomainState{DomainArgs: req.Inputs}}, nil
	}
	c, err := platformClient(ctx)
	if err != nil {
		return infer.CreateResponse[DomainState]{}, err
	}
	body := map[string]any{"name": req.Inputs.Name}
	if v := derefStr(req.Inputs.ProxyPath); v != "" {
		body["proxy_path"] = v
	}
	var out domainJSON
	if err := c.do(ctx, http.MethodPost, "/platform/applications/"+req.Inputs.ApplicationID+"/domains", body, &out); err != nil {
		return infer.CreateResponse[DomainState]{}, fmt.Errorf("clerk: create domain %q: %w", req.Inputs.Name, err)
	}
	return infer.CreateResponse[DomainState]{ID: req.Inputs.ApplicationID + "/" + out.ID, Output: out.state(req.Inputs)}, nil
}

func (r *Domain) Read(ctx context.Context, req infer.ReadRequest[DomainArgs, DomainState]) (infer.ReadResponse[DomainArgs, DomainState], error) {
	empty := infer.ReadResponse[DomainArgs, DomainState]{}
	appID, domainID, err := splitDomainID(req.ID)
	if err != nil {
		return empty, err
	}
	c, err := platformClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := getDomainJSON(ctx, c, appID, domainID)
	if isNotFound(err) {
		return empty, nil
	}
	if err != nil {
		return empty, fmt.Errorf("clerk: read domain %q: %w", req.ID, err)
	}
	args := DomainArgs{ApplicationID: appID, ProxyPath: req.Inputs.ProxyPath}
	if args.ProxyPath == nil {
		args.ProxyPath = req.State.ProxyPath
	}
	st := got.state(args)
	return infer.ReadResponse[DomainArgs, DomainState]{ID: req.ID, Inputs: st.DomainArgs, State: st}, nil
}

func (r *Domain) Delete(ctx context.Context, req infer.DeleteRequest[DomainState]) (infer.DeleteResponse, error) {
	appID, domainID, err := splitDomainID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	c, err := platformClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	err = c.do(ctx, http.MethodDelete, "/platform/applications/"+appID+"/domains/"+domainID, nil, nil)
	if err == nil || isNotFound(err) {
		return infer.DeleteResponse{}, nil
	}
	// The Platform API's domain delete endpoint has a known server-side
	// failure; the domain is removed with its application, so warn only.
	if ae, ok := err.(*apiError); ok && ae.Status == http.StatusInternalServerError {
		pgp.GetLogger(ctx).Warningf("clerk: the API answered HTTP 500 deleting domain %s; it is removed when its application is deleted: %v", req.ID, err)
		return infer.DeleteResponse{}, nil
	}
	return infer.DeleteResponse{}, fmt.Errorf("clerk: delete domain %q: %w", req.ID, err)
}

// GetDomain looks up a domain of a Clerk application.
type GetDomain struct{}

// GetDomainArgs selects the domain.
type GetDomainArgs struct {
	ApplicationID string `pulumi:"applicationId"`
	ID            string `pulumi:"domainId"`
}

// GetDomainResult is the looked-up domain.
type GetDomainResult struct {
	ID               string              `pulumi:"domainId"`
	ApplicationID    string              `pulumi:"applicationId"`
	Name             string              `pulumi:"name"`
	IsSatellite      bool                `pulumi:"isSatellite"`
	IsProviderDomain bool                `pulumi:"isProviderDomain"`
	FrontendAPIURL   string              `pulumi:"frontendApiUrl"`
	CNAMETargets     []DomainCNAMETarget `pulumi:"cnameTargets"`
}

func (f *GetDomain) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a domain of a Clerk application by application ID and domain ID through the Platform API (beta).")
}

func (a *GetDomainArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ApplicationID, "The ID of the application the domain belongs to.")
	ann.Describe(&a.ID, "The domain ID (without the application prefix).")
}

func (r *GetDomainResult) Annotate(ann infer.Annotator) {
	ann.Describe(&r.ID, "The domain ID.")
	ann.Describe(&r.ApplicationID, "The application ID.")
	ann.Describe(&r.Name, "The domain name.")
	ann.Describe(&r.IsSatellite, "Whether this is a satellite domain.")
	ann.Describe(&r.IsProviderDomain, "Whether this is a provider domain.")
	ann.Describe(&r.FrontendAPIURL, "The frontend API URL of this domain.")
	ann.Describe(&r.CNAMETargets, "The CNAME records to create in DNS for this domain.")
}

func (f *GetDomain) Invoke(ctx context.Context, req infer.FunctionRequest[GetDomainArgs]) (infer.FunctionResponse[GetDomainResult], error) {
	out := infer.FunctionResponse[GetDomainResult]{}
	if req.Input.ApplicationID == "" || req.Input.ID == "" {
		return out, fmt.Errorf("clerk: getDomain needs applicationId and id")
	}
	c, err := platformClient(ctx)
	if err != nil {
		return out, err
	}
	got, err := getDomainJSON(ctx, c, req.Input.ApplicationID, req.Input.ID)
	if err != nil {
		return out, lookupErr("domain", fmt.Sprintf("%q", req.Input.ApplicationID+"/"+req.Input.ID), err)
	}
	st := got.state(DomainArgs{ApplicationID: req.Input.ApplicationID})
	out.Output = GetDomainResult{ID: got.ID, ApplicationID: req.Input.ApplicationID, Name: got.Name, IsSatellite: st.IsSatellite,
		IsProviderDomain: st.IsProviderDomain, FrontendAPIURL: st.FrontendAPIURL, CNAMETargets: st.CNAMETargets}
	return out, nil
}
