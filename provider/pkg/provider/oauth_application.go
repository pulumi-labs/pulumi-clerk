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

// OauthApplication is a Clerk OAuth application: Clerk acting as an OAuth 2.0
// and OpenID Connect identity provider for third-party apps.
type OauthApplication struct{}

// OauthApplicationArgs are the inputs of an OauthApplication.
type OauthApplicationArgs struct {
	// Name is the application name.
	Name string `pulumi:"name"`
	// CallbackUrl is the OAuth redirect (callback) URL.
	CallbackUrl string `pulumi:"callbackUrl"`
	// Scopes is the space-separated list of OAuth scopes.
	Scopes string `pulumi:"scopes"`
	// Public marks a public (secret-less) client; it cannot be changed.
	Public bool `pulumi:"public" provider:"replaceOnChanges"`
	// ConsentScreenEnabled shows the consent screen to users.
	ConsentScreenEnabled *bool `pulumi:"consentScreenEnabled,optional"`
}

// OauthApplicationState is the stored state of an OauthApplication.
type OauthApplicationState struct {
	OauthApplicationArgs
	ClientId                      string `pulumi:"clientId"`
	ClientSecret                  string `pulumi:"clientSecret" provider:"secret"`
	DiscoveryUrl                  string `pulumi:"discoveryUrl"`
	AuthorizeUrl                  string `pulumi:"authorizeUrl"`
	TokenFetchUrl                 string `pulumi:"tokenFetchUrl"`
	UserInfoUrl                   string `pulumi:"userInfoUrl"`
	EffectiveScopes               string `pulumi:"effectiveScopes"`
	EffectiveConsentScreenEnabled bool   `pulumi:"effectiveConsentScreenEnabled"`
	CreatedAt                     string `pulumi:"createdAt"`
	UpdatedAt                     string `pulumi:"updatedAt"`
}

func (r *OauthApplication) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk OAuth application: lets Clerk act as an OAuth 2.0 and OpenID Connect identity provider for a third-party app. Whether the application is public cannot be changed, so changing it replaces the application.")
}

func (a *OauthApplicationArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "Name of the OAuth application.")
	ann.Describe(&a.CallbackUrl, "The OAuth callback (redirect) URL.")
	ann.Describe(&a.Scopes, "Space-separated OAuth scopes, for example \"profile email\". Clerk may reorder them and add implicit scopes such as offline_access; the scopes Clerk reports are in effectiveScopes and are not a change as long as every scope you asked for is present.")
	ann.Describe(&a.Public, "Whether this is a public OAuth application (one with no client secret). Changing this forces a new resource.")
	ann.Describe(&a.ConsentScreenEnabled, "Whether to show the consent screen to users. When unset Clerk applies its default, reported in effectiveConsentScreenEnabled.")
}

func (s *OauthApplicationState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.ClientId, "The OAuth client ID.")
	ann.Describe(&s.ClientSecret, "The OAuth client secret. Secret. Clerk returns it only when the application is created, so it is empty for an imported application.")
	ann.Describe(&s.DiscoveryUrl, "The OpenID Connect discovery URL.")
	ann.Describe(&s.AuthorizeUrl, "The OAuth authorize URL.")
	ann.Describe(&s.TokenFetchUrl, "The OAuth token endpoint URL.")
	ann.Describe(&s.UserInfoUrl, "The OAuth user info endpoint URL.")
	ann.Describe(&s.EffectiveScopes, "The scopes Clerk reports, which can be reordered and include implicit scopes.")
	ann.Describe(&s.EffectiveConsentScreenEnabled, "Whether the consent screen is enabled, including Clerk's default.")
	ann.Describe(&s.CreatedAt, "When the application was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the application was last updated (RFC 3339).")
}

var _ infer.CustomResource[OauthApplicationArgs, OauthApplicationState] = (*OauthApplication)(nil)
var _ infer.CustomUpdate[OauthApplicationArgs, OauthApplicationState] = (*OauthApplication)(nil)
var _ infer.CustomRead[OauthApplicationArgs, OauthApplicationState] = (*OauthApplication)(nil)
var _ infer.CustomDelete[OauthApplicationState] = (*OauthApplication)(nil)

// oauthApplicationJSON is the API shape.
type oauthApplicationJSON struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	ClientID             string  `json:"client_id"`
	Public               bool    `json:"public"`
	ConsentScreenEnabled bool    `json:"consent_screen_enabled"`
	Scopes               string  `json:"scopes"`
	CallbackURL          string  `json:"callback_url"`
	DiscoveryURL         string  `json:"discovery_url"`
	AuthorizeURL         string  `json:"authorize_url"`
	TokenFetchURL        string  `json:"token_fetch_url"`
	UserInfoURL          string  `json:"user_info_url"`
	CreatedAt            int64   `json:"created_at"`
	UpdatedAt            int64   `json:"updated_at"`
	ClientSecret         *string `json:"client_secret,omitempty"`
}

func (j oauthApplicationJSON) outputs(in OauthApplicationArgs, secret string) OauthApplicationState {
	return OauthApplicationState{
		OauthApplicationArgs:          in,
		ClientId:                      j.ClientID,
		ClientSecret:                  secret,
		DiscoveryUrl:                  j.DiscoveryURL,
		AuthorizeUrl:                  j.AuthorizeURL,
		TokenFetchUrl:                 j.TokenFetchURL,
		UserInfoUrl:                   j.UserInfoURL,
		EffectiveScopes:               j.Scopes,
		EffectiveConsentScreenEnabled: j.ConsentScreenEnabled,
		CreatedAt:                     millisToRFC3339(j.CreatedAt),
		UpdatedAt:                     millisToRFC3339(j.UpdatedAt),
	}
}

// scopesSatisfied reports whether every scope in want is present in got,
// ignoring order and the extra implicit scopes Clerk may add.
func scopesSatisfied(want, got string) bool {
	have := map[string]bool{}
	for _, s := range strings.Fields(got) {
		have[s] = true
	}
	for _, s := range strings.Fields(want) {
		if !have[s] {
			return false
		}
	}
	return true
}

func oauthApplicationBody(in OauthApplicationArgs, create bool) map[string]any {
	body := map[string]any{"name": in.Name, "callback_url": in.CallbackUrl, "scopes": in.Scopes}
	if create {
		body["public"] = in.Public
	}
	if in.ConsentScreenEnabled != nil {
		body["consent_screen_enabled"] = *in.ConsentScreenEnabled
	}
	return body
}

func getOauthApplication(ctx context.Context, c *client, id string) (*oauthApplicationJSON, error) {
	var out oauthApplicationJSON
	if err := c.do(ctx, http.MethodGet, "/oauth_applications/"+id, nil, &out); err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func (r *OauthApplication) Create(ctx context.Context, req infer.CreateRequest[OauthApplicationArgs]) (infer.CreateResponse[OauthApplicationState], error) {
	if req.DryRun {
		return infer.CreateResponse[OauthApplicationState]{Output: OauthApplicationState{OauthApplicationArgs: req.Inputs}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[OauthApplicationState]{}, err
	}
	var out oauthApplicationJSON
	if err := c.do(ctx, http.MethodPost, "/oauth_applications", oauthApplicationBody(req.Inputs, true), &out); err != nil {
		return infer.CreateResponse[OauthApplicationState]{}, fmt.Errorf("clerk: create OAuth application %q: %w", req.Inputs.Name, err)
	}
	// The client secret exists only in this response, so it goes straight into state.
	return infer.CreateResponse[OauthApplicationState]{ID: out.ID, Output: out.outputs(req.Inputs, derefStr(out.ClientSecret))}, nil
}

var _ infer.CustomDiff[OauthApplicationArgs, OauthApplicationState] = (*OauthApplication)(nil)

// Diff updates in place and replaces only on a change of public. Scopes are
// compared the way Clerk keeps them: order and the implicit scopes Clerk adds
// are not a change, so an imported application whose stored scopes are
// "email offline_access profile" matches a program that declares "profile
// email" (#1563). An unset consentScreenEnabled is unmanaged.
func (r *OauthApplication) Diff(_ context.Context, req infer.DiffRequest[OauthApplicationArgs, OauthApplicationState]) (infer.DiffResponse, error) {
	diff := map[string]pgp.PropertyDiff{}
	in, st := req.Inputs, req.State
	if in.Name != st.Name {
		diff["name"] = pgp.PropertyDiff{Kind: pgp.Update, InputDiff: true}
	}
	if in.CallbackUrl != st.CallbackUrl {
		diff["callbackUrl"] = pgp.PropertyDiff{Kind: pgp.Update, InputDiff: true}
	}
	if !scopesSatisfied(in.Scopes, st.EffectiveScopes) && !scopesSatisfied(in.Scopes, st.Scopes) {
		diff["scopes"] = pgp.PropertyDiff{Kind: pgp.Update, InputDiff: true}
	}
	if in.Public != st.Public {
		diff["public"] = pgp.PropertyDiff{Kind: pgp.UpdateReplace, InputDiff: true}
	}
	if in.ConsentScreenEnabled != nil && *in.ConsentScreenEnabled != st.EffectiveConsentScreenEnabled {
		diff["consentScreenEnabled"] = pgp.PropertyDiff{Kind: pgp.Update, InputDiff: true}
	}
	return infer.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}

func (r *OauthApplication) Read(ctx context.Context, req infer.ReadRequest[OauthApplicationArgs, OauthApplicationState]) (infer.ReadResponse[OauthApplicationArgs, OauthApplicationState], error) {
	empty := infer.ReadResponse[OauthApplicationArgs, OauthApplicationState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := getOauthApplication(ctx, c, req.ID)
	if err != nil {
		return empty, fmt.Errorf("clerk: read OAuth application %q: %w", req.ID, err)
	}
	if got == nil {
		if err := notFoundOnImport("oauth application", req.ID, req.Inputs, req.State); err != nil {
			return empty, err
		}
		return empty, nil
	}
	in := req.Inputs
	imported := in.Name == ""
	in.Name, in.CallbackUrl, in.Public = got.Name, got.CallbackURL, got.Public
	// Clerk reorders scopes and adds implicit ones: keep the program's value
	// while it is still satisfied, otherwise report what Clerk has.
	if imported || !scopesSatisfied(in.Scopes, got.Scopes) {
		in.Scopes = got.Scopes
	}
	// An import adopts consentScreenEnabled only when it is not Clerk's default:
	// the default stays in effectiveConsentScreenEnabled (#1563), a real value is
	// adopted so a program declaring it plans no change (#1801).
	if !imported && in.ConsentScreenEnabled != nil {
		v := got.ConsentScreenEnabled
		in.ConsentScreenEnabled = &v
	} else if imported {
		in.ConsentScreenEnabled = adoptIfNotDefault(got.ConsentScreenEnabled, clerkDefaultConsentScreenEnabled)
	}
	// The client secret is only ever returned at creation: keep it.
	st := got.outputs(in, req.State.ClientSecret)
	return infer.ReadResponse[OauthApplicationArgs, OauthApplicationState]{ID: got.ID, Inputs: in, State: st}, nil
}

func (r *OauthApplication) Update(ctx context.Context, req infer.UpdateRequest[OauthApplicationArgs, OauthApplicationState]) (infer.UpdateResponse[OauthApplicationState], error) {
	if req.DryRun {
		st := req.State
		st.OauthApplicationArgs = req.Inputs
		return infer.UpdateResponse[OauthApplicationState]{Output: st}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[OauthApplicationState]{}, err
	}
	var out oauthApplicationJSON
	if err := c.do(ctx, http.MethodPatch, "/oauth_applications/"+req.ID, oauthApplicationBody(req.Inputs, false), &out); err != nil {
		return infer.UpdateResponse[OauthApplicationState]{}, fmt.Errorf("clerk: update OAuth application %q: %w", req.ID, err)
	}
	return infer.UpdateResponse[OauthApplicationState]{Output: out.outputs(req.Inputs, req.State.ClientSecret)}, nil
}

func (r *OauthApplication) Delete(ctx context.Context, req infer.DeleteRequest[OauthApplicationState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/oauth_applications/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete OAuth application %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetOauthApplication looks up an OAuth application by ID, client ID or name.
type GetOauthApplication struct{}

// GetOauthApplicationArgs selects the application; set exactly one of the fields.
type GetOauthApplicationArgs struct {
	ID       *string `pulumi:"oauthApplicationId,optional"`
	ClientId *string `pulumi:"clientId,optional"`
	Name     *string `pulumi:"name,optional"`
}

// GetOauthApplicationResult is the looked-up application. It never includes the client secret.
type GetOauthApplicationResult struct {
	ID                   string `pulumi:"oauthApplicationId"`
	Name                 string `pulumi:"name"`
	ClientId             string `pulumi:"clientId"`
	CallbackUrl          string `pulumi:"callbackUrl"`
	Scopes               string `pulumi:"scopes"`
	Public               bool   `pulumi:"public"`
	ConsentScreenEnabled bool   `pulumi:"consentScreenEnabled"`
	DiscoveryUrl         string `pulumi:"discoveryUrl"`
	AuthorizeUrl         string `pulumi:"authorizeUrl"`
	TokenFetchUrl        string `pulumi:"tokenFetchUrl"`
	UserInfoUrl          string `pulumi:"userInfoUrl"`
	CreatedAt            string `pulumi:"createdAt"`
	UpdatedAt            string `pulumi:"updatedAt"`
}

func (f *GetOauthApplication) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk OAuth application by ID, client ID or exact name. The client secret is not returned.")
}

func (a *GetOauthApplicationArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The OAuth application ID. Set exactly one of oauthApplicationId, clientId or name.")
	ann.Describe(&a.ClientId, "The OAuth client ID. Set exactly one of oauthApplicationId, clientId or name.")
	ann.Describe(&a.Name, "The application name. Set exactly one of oauthApplicationId, clientId or name.")
}

func (f *GetOauthApplication) Invoke(ctx context.Context, req infer.FunctionRequest[GetOauthApplicationArgs]) (infer.FunctionResponse[GetOauthApplicationResult], error) {
	out := infer.FunctionResponse[GetOauthApplicationResult]{}
	id, clientID, name := derefStr(req.Input.ID), derefStr(req.Input.ClientId), derefStr(req.Input.Name)
	set := 0
	for _, v := range []string{id, clientID, name} {
		if v != "" {
			set++
		}
	}
	if set != 1 {
		return out, fmt.Errorf("clerk: getOauthApplication needs exactly one of oauthApplicationId, clientId or name")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	var got *oauthApplicationJSON
	if id != "" {
		got, err = getOauthApplication(ctx, c, id)
	} else {
		var items []oauthApplicationJSON
		items, err = listAll[oauthApplicationJSON](ctx, c, "/oauth_applications", nil)
		for i := range items {
			if (clientID != "" && items[i].ClientID == clientID) || (name != "" && items[i].Name == name) {
				got = &items[i]
				break
			}
		}
	}
	what := id + clientID + name
	if err != nil {
		return out, lookupErr("OAuth application", what, err)
	}
	if got == nil {
		return out, lookupErr("OAuth application", fmt.Sprintf("%q", what), &apiError{Status: http.StatusNotFound})
	}
	out.Output = GetOauthApplicationResult{ID: got.ID, Name: got.Name, ClientId: got.ClientID, CallbackUrl: got.CallbackURL,
		Scopes: got.Scopes, Public: got.Public, ConsentScreenEnabled: got.ConsentScreenEnabled, DiscoveryUrl: got.DiscoveryURL,
		AuthorizeUrl: got.AuthorizeURL, TokenFetchUrl: got.TokenFetchURL, UserInfoUrl: got.UserInfoURL,
		CreatedAt: millisToRFC3339(got.CreatedAt), UpdatedAt: millisToRFC3339(got.UpdatedAt)}
	return out, nil
}
