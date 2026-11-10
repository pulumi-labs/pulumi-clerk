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

// SamlConnection is a SAML single sign-on connection on a Clerk instance.
type SamlConnection struct{}

// SamlConnectionAttributeMapping maps SAML assertion attribute names to Clerk
// user fields.
type SamlConnectionAttributeMapping struct {
	UserID       string `pulumi:"userId"`
	EmailAddress string `pulumi:"emailAddress"`
	FirstName    string `pulumi:"firstName"`
	LastName     string `pulumi:"lastName"`
}

// SamlConnectionArgs are the inputs of a SamlConnection.
type SamlConnectionArgs struct {
	Name                             string                          `pulumi:"name"`
	Domain                           string                          `pulumi:"domain"`
	ProviderType                     string                          `pulumi:"providerType" provider:"replaceOnChanges"`
	OrganizationID                   *string                         `pulumi:"organizationId,optional"`
	IdpEntityID                      *string                         `pulumi:"idpEntityId,optional"`
	IdpSsoURL                        *string                         `pulumi:"idpSsoUrl,optional"`
	IdpCertificate                   *string                         `pulumi:"idpCertificate,optional"`
	IdpMetadataURL                   *string                         `pulumi:"idpMetadataUrl,optional"`
	IdpMetadata                      *string                         `pulumi:"idpMetadata,optional"`
	AttributeMapping                 *SamlConnectionAttributeMapping `pulumi:"attributeMapping,optional"`
	Active                           *bool                           `pulumi:"active,optional"`
	SyncUserAttributes               *bool                           `pulumi:"syncUserAttributes,optional"`
	AllowSubdomains                  *bool                           `pulumi:"allowSubdomains,optional"`
	AllowIdpInitiated                *bool                           `pulumi:"allowIdpInitiated,optional"`
	DisableAdditionalIdentifications *bool                           `pulumi:"disableAdditionalIdentifications,optional"`
	ForceAuthn                       *bool                           `pulumi:"forceAuthn,optional"`
}

// SamlConnectionEffective holds the values Clerk actually applies, including
// the ones it defaults or derives when the matching input is left unset.
type SamlConnectionEffective struct {
	IdpEntityID                      *string                        `pulumi:"idpEntityId,optional"`
	IdpSsoURL                        *string                        `pulumi:"idpSsoUrl,optional"`
	IdpCertificate                   *string                        `pulumi:"idpCertificate,optional"`
	AttributeMapping                 SamlConnectionAttributeMapping `pulumi:"attributeMapping"`
	Active                           bool                           `pulumi:"active"`
	SyncUserAttributes               bool                           `pulumi:"syncUserAttributes"`
	AllowSubdomains                  bool                           `pulumi:"allowSubdomains"`
	AllowIdpInitiated                bool                           `pulumi:"allowIdpInitiated"`
	DisableAdditionalIdentifications bool                           `pulumi:"disableAdditionalIdentifications"`
	ForceAuthn                       bool                           `pulumi:"forceAuthn"`
}

// SamlConnectionState is the stored state of a SamlConnection. The input part
// keeps what the user set (unset stays unset, so Clerk's defaults never show as
// changes); Effective carries the values Clerk reports.
type SamlConnectionState struct {
	SamlConnectionArgs
	Effective     SamlConnectionEffective `pulumi:"effective"`
	AcsURL        string                  `pulumi:"acsUrl"`
	SpEntityID    string                  `pulumi:"spEntityId"`
	SpMetadataURL string                  `pulumi:"spMetadataUrl"`
	UserCount     int                     `pulumi:"userCount"`
	CreatedAt     string                  `pulumi:"createdAt"`
	UpdatedAt     string                  `pulumi:"updatedAt"`
}

func (r *SamlConnection) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk SAML connection for enterprise single sign-on. The provider type cannot be changed after creation (changing it replaces the connection). Import with the connection ID, for example saml_...")
}

func (m *SamlConnectionAttributeMapping) Annotate(a infer.Annotator) {
	a.Describe(&m.UserID, "SAML attribute name for the user ID.")
	a.Describe(&m.EmailAddress, "SAML attribute name for the email address.")
	a.Describe(&m.FirstName, "SAML attribute name for the first name.")
	a.Describe(&m.LastName, "SAML attribute name for the last name.")
}

func (a *SamlConnectionArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "Name of the SAML connection.")
	ann.Describe(&a.Domain, "Domain the SAML connection applies to.")
	ann.Describe(&a.ProviderType, "SAML provider type, for example saml_custom, saml_okta or saml_google. Changing this forces a new resource.")
	ann.Describe(&a.OrganizationID, "ID of the organization to associate with the connection.")
	ann.Describe(&a.IdpEntityID, "Entity ID of the identity provider.")
	ann.Describe(&a.IdpSsoURL, "Single sign-on URL of the identity provider.")
	ann.Describe(&a.IdpCertificate, "X.509 certificate of the identity provider (a public certificate, not a secret).")
	ann.Describe(&a.IdpMetadataURL, "Metadata URL of the identity provider.")
	ann.Describe(&a.IdpMetadata, "Raw metadata XML of the identity provider.")
	ann.Describe(&a.AttributeMapping, "Mapping of SAML attributes to Clerk user fields. When omitted Clerk applies its defaults.")
	ann.Describe(&a.Active, "Whether the connection is active. Clerk decides the default when omitted.")
	ann.Describe(&a.SyncUserAttributes, "Whether to sync user attributes from the identity provider on every sign-in.")
	ann.Describe(&a.AllowSubdomains, "Whether subdomains of the domain are allowed.")
	ann.Describe(&a.AllowIdpInitiated, "Whether IdP-initiated sign-in is allowed.")
	ann.Describe(&a.DisableAdditionalIdentifications, "Whether to disable additional identifications.")
	ann.Describe(&a.ForceAuthn, "Whether to force re-authentication on every sign-in.")
}

func (e *SamlConnectionEffective) Annotate(ann infer.Annotator) {
	ann.Describe(&e.IdpEntityID, "The identity provider entity ID Clerk uses (derived from the metadata when not set).")
	ann.Describe(&e.IdpSsoURL, "The identity provider single sign-on URL Clerk uses (derived from the metadata when not set).")
	ann.Describe(&e.IdpCertificate, "The identity provider certificate Clerk uses (derived from the metadata when not set).")
	ann.Describe(&e.AttributeMapping, "The attribute mapping Clerk applies (its defaults when attributeMapping is not set).")
	ann.Describe(&e.Active, "Whether the connection is active.")
	ann.Describe(&e.SyncUserAttributes, "Whether user attributes are synced on every sign-in.")
	ann.Describe(&e.AllowSubdomains, "Whether subdomains are allowed.")
	ann.Describe(&e.AllowIdpInitiated, "Whether IdP-initiated sign-in is allowed.")
	ann.Describe(&e.DisableAdditionalIdentifications, "Whether additional identifications are disabled.")
	ann.Describe(&e.ForceAuthn, "Whether re-authentication is forced on every sign-in.")
}

func (s *SamlConnectionState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.Effective, "The values Clerk applies, including defaults for inputs you left unset.")
	ann.Describe(&s.AcsURL, "Assertion Consumer Service URL to configure in the identity provider.")
	ann.Describe(&s.SpEntityID, "Service provider entity ID to configure in the identity provider.")
	ann.Describe(&s.SpMetadataURL, "Service provider metadata URL.")
	ann.Describe(&s.UserCount, "Number of users signed in through this connection.")
	ann.Describe(&s.CreatedAt, "When the connection was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the connection was last updated (RFC 3339).")
}

var _ infer.CustomResource[SamlConnectionArgs, SamlConnectionState] = (*SamlConnection)(nil)
var _ infer.CustomUpdate[SamlConnectionArgs, SamlConnectionState] = (*SamlConnection)(nil)
var _ infer.CustomRead[SamlConnectionArgs, SamlConnectionState] = (*SamlConnection)(nil)
var _ infer.CustomDelete[SamlConnectionState] = (*SamlConnection)(nil)

// samlConnectionJSON is the API shape.
type samlConnectionJSON struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Domain           string  `json:"domain"`
	Provider         string  `json:"provider"`
	OrganizationID   *string `json:"organization_id"`
	IdpEntityID      *string `json:"idp_entity_id"`
	IdpSsoURL        *string `json:"idp_sso_url"`
	IdpCertificate   *string `json:"idp_certificate"`
	IdpMetadataURL   *string `json:"idp_metadata_url"`
	IdpMetadata      *string `json:"idp_metadata"`
	AcsURL           string  `json:"acs_url"`
	SPEntityID       string  `json:"sp_entity_id"`
	SPMetadataURL    string  `json:"sp_metadata_url"`
	AttributeMapping struct {
		UserID       string `json:"user_id"`
		EmailAddress string `json:"email_address"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
	} `json:"attribute_mapping"`
	Active                           bool  `json:"active"`
	UserCount                        int64 `json:"user_count"`
	SyncUserAttributes               bool  `json:"sync_user_attributes"`
	AllowSubdomains                  bool  `json:"allow_subdomains"`
	AllowIdpInitiated                bool  `json:"allow_idp_initiated"`
	DisableAdditionalIdentifications bool  `json:"disable_additional_identifications"`
	ForceAuthn                       bool  `json:"force_authn"`
	CreatedAt                        int64 `json:"created_at"`
	UpdatedAt                        int64 `json:"updated_at"`
}

func samlBoolPtr(b bool) *bool { return &b }

// state renders the API object; args is the input part to keep (what the user
// set), and the server's values go to Effective.
func (j samlConnectionJSON) state(args SamlConnectionArgs) SamlConnectionState {
	return SamlConnectionState{
		SamlConnectionArgs: args,
		Effective: SamlConnectionEffective{
			IdpEntityID:    j.IdpEntityID,
			IdpSsoURL:      j.IdpSsoURL,
			IdpCertificate: j.IdpCertificate,
			AttributeMapping: SamlConnectionAttributeMapping{
				UserID:       j.AttributeMapping.UserID,
				EmailAddress: j.AttributeMapping.EmailAddress,
				FirstName:    j.AttributeMapping.FirstName,
				LastName:     j.AttributeMapping.LastName,
			},
			Active:                           j.Active,
			SyncUserAttributes:               j.SyncUserAttributes,
			AllowSubdomains:                  j.AllowSubdomains,
			AllowIdpInitiated:                j.AllowIdpInitiated,
			DisableAdditionalIdentifications: j.DisableAdditionalIdentifications,
			ForceAuthn:                       j.ForceAuthn,
		},
		AcsURL:        j.AcsURL,
		SpEntityID:    j.SPEntityID,
		SpMetadataURL: j.SPMetadataURL,
		UserCount:     int(j.UserCount),
		CreatedAt:     millisToRFC3339(j.CreatedAt),
		UpdatedAt:     millisToRFC3339(j.UpdatedAt),
	}
}

// samlKeepUnset returns nil when the prior input was unset, else the server
// value, so that server-side defaults never show up as input changes.
func samlKeepUnset[T any](prior, server *T) *T {
	if prior == nil {
		return nil
	}
	return server
}

// samlKeepPrior prefers the server value but falls back to the prior input when
// Clerk does not echo the field back (the IdP metadata may be write-only).
func samlKeepPrior(prior, server *string) *string {
	if prior == nil {
		return nil
	}
	if server == nil {
		return prior
	}
	return server
}

// refreshedArgs derives the inputs a refresh reports: the server's value for
// everything the user set, nothing for optional inputs they left unset. An
// import (no prior inputs) adopts every server value.
func (j samlConnectionJSON) refreshedArgs(prior SamlConnectionArgs) SamlConnectionArgs {
	am := &SamlConnectionAttributeMapping{
		UserID: j.AttributeMapping.UserID, EmailAddress: j.AttributeMapping.EmailAddress,
		FirstName: j.AttributeMapping.FirstName, LastName: j.AttributeMapping.LastName,
	}
	in := SamlConnectionArgs{
		Name: j.Name, Domain: j.Domain, ProviderType: j.Provider,
		OrganizationID: j.OrganizationID, IdpEntityID: j.IdpEntityID, IdpSsoURL: j.IdpSsoURL,
		IdpCertificate: j.IdpCertificate, IdpMetadataURL: j.IdpMetadataURL, IdpMetadata: j.IdpMetadata,
		AttributeMapping: am, Active: samlBoolPtr(j.Active), SyncUserAttributes: samlBoolPtr(j.SyncUserAttributes),
		AllowSubdomains: samlBoolPtr(j.AllowSubdomains), AllowIdpInitiated: samlBoolPtr(j.AllowIdpInitiated),
		DisableAdditionalIdentifications: samlBoolPtr(j.DisableAdditionalIdentifications), ForceAuthn: samlBoolPtr(j.ForceAuthn),
	}
	if prior.Name == "" && prior.Domain == "" {
		return in
	}
	in.OrganizationID = samlKeepUnset(prior.OrganizationID, in.OrganizationID)
	in.IdpEntityID = samlKeepUnset(prior.IdpEntityID, in.IdpEntityID)
	in.IdpSsoURL = samlKeepUnset(prior.IdpSsoURL, in.IdpSsoURL)
	in.IdpCertificate = samlKeepUnset(prior.IdpCertificate, in.IdpCertificate)
	in.IdpMetadataURL = samlKeepPrior(prior.IdpMetadataURL, j.IdpMetadataURL)
	in.IdpMetadata = samlKeepPrior(prior.IdpMetadata, j.IdpMetadata)
	in.AttributeMapping = samlKeepUnset(prior.AttributeMapping, in.AttributeMapping)
	in.Active = samlKeepUnset(prior.Active, in.Active)
	in.SyncUserAttributes = samlKeepUnset(prior.SyncUserAttributes, in.SyncUserAttributes)
	in.AllowSubdomains = samlKeepUnset(prior.AllowSubdomains, in.AllowSubdomains)
	in.AllowIdpInitiated = samlKeepUnset(prior.AllowIdpInitiated, in.AllowIdpInitiated)
	in.DisableAdditionalIdentifications = samlKeepUnset(prior.DisableAdditionalIdentifications, in.DisableAdditionalIdentifications)
	in.ForceAuthn = samlKeepUnset(prior.ForceAuthn, in.ForceAuthn)
	return in
}

func samlSet[T any](body map[string]any, key string, v *T) {
	if v != nil {
		body[key] = *v
	}
}

func samlAttributeMappingBody(m *SamlConnectionAttributeMapping) map[string]any {
	return map[string]any{"user_id": m.UserID, "email_address": m.EmailAddress, "first_name": m.FirstName, "last_name": m.LastName}
}

// samlUpdateBody holds everything the PATCH endpoint accepts.
func samlUpdateBody(in SamlConnectionArgs) map[string]any {
	body := map[string]any{"name": in.Name, "domain": in.Domain}
	samlSet(body, "organization_id", in.OrganizationID)
	samlSet(body, "idp_entity_id", in.IdpEntityID)
	samlSet(body, "idp_sso_url", in.IdpSsoURL)
	samlSet(body, "idp_certificate", in.IdpCertificate)
	samlSet(body, "idp_metadata_url", in.IdpMetadataURL)
	samlSet(body, "idp_metadata", in.IdpMetadata)
	if in.AttributeMapping != nil {
		body["attribute_mapping"] = samlAttributeMappingBody(in.AttributeMapping)
	}
	samlSet(body, "active", in.Active)
	samlSet(body, "sync_user_attributes", in.SyncUserAttributes)
	samlSet(body, "allow_subdomains", in.AllowSubdomains)
	samlSet(body, "allow_idp_initiated", in.AllowIdpInitiated)
	samlSet(body, "disable_additional_identifications", in.DisableAdditionalIdentifications)
	samlSet(body, "force_authn", in.ForceAuthn)
	return body
}

// dryRunState previews a state from inputs without calling the API.
func (a SamlConnectionArgs) dryRunState() SamlConnectionState {
	return SamlConnectionState{SamlConnectionArgs: a}
}

func (r *SamlConnection) Create(ctx context.Context, req infer.CreateRequest[SamlConnectionArgs]) (infer.CreateResponse[SamlConnectionState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[SamlConnectionState]{Output: in.dryRunState()}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[SamlConnectionState]{}, err
	}
	// The create endpoint takes the connection definition; the activation and
	// sync flags can only be set by a follow-up update.
	body := map[string]any{"name": in.Name, "domain": in.Domain, "provider": in.ProviderType}
	samlSet(body, "organization_id", in.OrganizationID)
	samlSet(body, "idp_entity_id", in.IdpEntityID)
	samlSet(body, "idp_sso_url", in.IdpSsoURL)
	samlSet(body, "idp_certificate", in.IdpCertificate)
	samlSet(body, "idp_metadata_url", in.IdpMetadataURL)
	samlSet(body, "idp_metadata", in.IdpMetadata)
	if in.AttributeMapping != nil {
		body["attribute_mapping"] = samlAttributeMappingBody(in.AttributeMapping)
	}
	samlSet(body, "force_authn", in.ForceAuthn)
	var out samlConnectionJSON
	if err := c.do(ctx, http.MethodPost, "/saml_connections", body, &out); err != nil {
		return infer.CreateResponse[SamlConnectionState]{}, fmt.Errorf("clerk: create SAML connection %q: %w", in.Name, err)
	}
	st := out.state(in)
	resp := infer.CreateResponse[SamlConnectionState]{ID: out.ID, Output: st}
	if in.Active != nil || in.SyncUserAttributes != nil || in.AllowSubdomains != nil || in.AllowIdpInitiated != nil || in.DisableAdditionalIdentifications != nil {
		patch := map[string]any{}
		samlSet(patch, "active", in.Active)
		samlSet(patch, "sync_user_attributes", in.SyncUserAttributes)
		samlSet(patch, "allow_subdomains", in.AllowSubdomains)
		samlSet(patch, "allow_idp_initiated", in.AllowIdpInitiated)
		samlSet(patch, "disable_additional_identifications", in.DisableAdditionalIdentifications)
		var upd samlConnectionJSON
		if err := c.do(ctx, http.MethodPatch, "/saml_connections/"+out.ID, patch, &upd); err != nil {
			return resp, infer.ResourceInitFailedError{Reasons: []string{fmt.Sprintf("SAML connection %s was created but its settings could not be applied: %s", out.ID, err)}}
		}
		resp.Output = upd.state(in)
	}
	return resp, nil
}

func (r *SamlConnection) Update(ctx context.Context, req infer.UpdateRequest[SamlConnectionArgs, SamlConnectionState]) (infer.UpdateResponse[SamlConnectionState], error) {
	if req.DryRun {
		st := req.State
		st.SamlConnectionArgs = req.Inputs
		return infer.UpdateResponse[SamlConnectionState]{Output: st}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[SamlConnectionState]{}, err
	}
	var out samlConnectionJSON
	if err := c.do(ctx, http.MethodPatch, "/saml_connections/"+req.ID, samlUpdateBody(req.Inputs), &out); err != nil {
		return infer.UpdateResponse[SamlConnectionState]{}, fmt.Errorf("clerk: update SAML connection %q: %w", req.ID, err)
	}
	return infer.UpdateResponse[SamlConnectionState]{Output: out.state(req.Inputs)}, nil
}

func (r *SamlConnection) Read(ctx context.Context, req infer.ReadRequest[SamlConnectionArgs, SamlConnectionState]) (infer.ReadResponse[SamlConnectionArgs, SamlConnectionState], error) {
	empty := infer.ReadResponse[SamlConnectionArgs, SamlConnectionState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	var out samlConnectionJSON
	if err := c.do(ctx, http.MethodGet, "/saml_connections/"+req.ID, nil, &out); err != nil {
		if isNotFound(err) {
			if err := notFoundOnImport("saml connection", req.ID, req.Inputs, req.State); err != nil {
				return empty, err
			}
			return empty, nil
		}
		return empty, fmt.Errorf("clerk: read SAML connection %q: %w", req.ID, err)
	}
	// A refresh has the previous inputs (or at least the previous state); an
	// import has neither.
	prior := req.Inputs
	if prior.Name == "" && prior.Domain == "" {
		prior = req.State.SamlConnectionArgs
	}
	args := out.refreshedArgs(prior)
	return infer.ReadResponse[SamlConnectionArgs, SamlConnectionState]{ID: out.ID, Inputs: args, State: out.state(args)}, nil
}

func (r *SamlConnection) Delete(ctx context.Context, req infer.DeleteRequest[SamlConnectionState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/saml_connections/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete SAML connection %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetSamlConnection looks up a SAML connection by ID or domain.
type GetSamlConnection struct{}

// GetSamlConnectionArgs selects the connection; set exactly one of the fields.
type GetSamlConnectionArgs struct {
	ID     *string `pulumi:"samlConnectionId,optional"`
	Domain *string `pulumi:"domain,optional"`
}

// GetSamlConnectionResult is the looked-up connection.
type GetSamlConnectionResult struct {
	ID                               string                         `pulumi:"samlConnectionId"`
	Name                             string                         `pulumi:"name"`
	Domain                           string                         `pulumi:"domain"`
	ProviderType                     string                         `pulumi:"providerType"`
	OrganizationID                   *string                        `pulumi:"organizationId,optional"`
	IdpEntityID                      *string                        `pulumi:"idpEntityId,optional"`
	IdpSsoURL                        *string                        `pulumi:"idpSsoUrl,optional"`
	IdpCertificate                   *string                        `pulumi:"idpCertificate,optional"`
	IdpMetadataURL                   *string                        `pulumi:"idpMetadataUrl,optional"`
	AttributeMapping                 SamlConnectionAttributeMapping `pulumi:"attributeMapping"`
	Active                           bool                           `pulumi:"active"`
	SyncUserAttributes               bool                           `pulumi:"syncUserAttributes"`
	AllowSubdomains                  bool                           `pulumi:"allowSubdomains"`
	AllowIdpInitiated                bool                           `pulumi:"allowIdpInitiated"`
	DisableAdditionalIdentifications bool                           `pulumi:"disableAdditionalIdentifications"`
	ForceAuthn                       bool                           `pulumi:"forceAuthn"`
	AcsURL                           string                         `pulumi:"acsUrl"`
	SpEntityID                       string                         `pulumi:"spEntityId"`
	SpMetadataURL                    string                         `pulumi:"spMetadataUrl"`
	UserCount                        int                            `pulumi:"userCount"`
	CreatedAt                        string                         `pulumi:"createdAt"`
	UpdatedAt                        string                         `pulumi:"updatedAt"`
}

func (f *GetSamlConnection) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk SAML connection by ID or by domain.")
}

func (a *GetSamlConnectionArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The SAML connection ID. Set either id or domain.")
	ann.Describe(&a.Domain, "The domain of the SAML connection. Set either id or domain.")
}

func (f *GetSamlConnection) Invoke(ctx context.Context, req infer.FunctionRequest[GetSamlConnectionArgs]) (infer.FunctionResponse[GetSamlConnectionResult], error) {
	out := infer.FunctionResponse[GetSamlConnectionResult]{}
	id, domain := derefStr(req.Input.ID), derefStr(req.Input.Domain)
	if (id == "") == (domain == "") {
		return out, fmt.Errorf("clerk: getSamlConnection needs exactly one of id or domain")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	var got *samlConnectionJSON
	if id != "" {
		var one samlConnectionJSON
		if err := c.do(ctx, http.MethodGet, "/saml_connections/"+id, nil, &one); err != nil {
			return out, lookupErr("SAML connection", fmt.Sprintf("%q", id), err)
		}
		got = &one
	} else {
		items, err := listAll[samlConnectionJSON](ctx, c, "/saml_connections", url.Values{"query": {domain}})
		if err != nil {
			return out, lookupErr("SAML connection", fmt.Sprintf("%q", domain), err)
		}
		for i := range items {
			if items[i].Domain == domain {
				got = &items[i]
				break
			}
		}
		if got == nil {
			return out, lookupErr("SAML connection", fmt.Sprintf("for domain %q", domain), &apiError{Status: http.StatusNotFound})
		}
	}
	st := got.state(SamlConnectionArgs{})
	m := st.Effective.AttributeMapping
	out.Output = GetSamlConnectionResult{
		ID: got.ID, Name: got.Name, Domain: got.Domain, ProviderType: got.Provider,
		OrganizationID: got.OrganizationID, IdpEntityID: got.IdpEntityID, IdpSsoURL: got.IdpSsoURL,
		IdpCertificate: got.IdpCertificate, IdpMetadataURL: got.IdpMetadataURL, AttributeMapping: m,
		Active: got.Active, SyncUserAttributes: got.SyncUserAttributes, AllowSubdomains: got.AllowSubdomains,
		AllowIdpInitiated: got.AllowIdpInitiated, DisableAdditionalIdentifications: got.DisableAdditionalIdentifications,
		ForceAuthn: got.ForceAuthn, AcsURL: got.AcsURL, SpEntityID: got.SPEntityID, SpMetadataURL: got.SPMetadataURL,
		UserCount: st.UserCount, CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt,
	}
	return out, nil
}
