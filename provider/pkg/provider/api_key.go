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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// ApiKey is a Clerk API key that a user, organization or machine can use to
// authenticate machine-to-machine requests.
type ApiKey struct{}

// ApiKeyArgs are the inputs of an ApiKey.
type ApiKeyArgs struct {
	// Name is the key name. The API cannot rename a key.
	Name string `pulumi:"name" provider:"replaceOnChanges"`
	// Type is the key type; it cannot be changed.
	Type *string `pulumi:"type,optional" provider:"replaceOnChanges"`
	// Subject is the user, organization, machine or SCIM subject the key acts for.
	Subject *string `pulumi:"subject,optional"`
	// Description is a free-form description.
	Description *string `pulumi:"description,optional"`
	// Claims is a JSON object of custom claims.
	Claims *string `pulumi:"claims,optional"`
	// Scopes are the scopes granted to the key.
	Scopes []string `pulumi:"scopes,optional"`
	// CreatedBy is the user that created the key; it can only be set at creation.
	CreatedBy *string `pulumi:"createdBy,optional" provider:"replaceOnChanges"`
	// SecondsUntilExpiration sets the key to expire this many seconds from now.
	SecondsUntilExpiration *int `pulumi:"secondsUntilExpiration,optional"`
}

// ApiKeyState is the stored state of an ApiKey.
type ApiKeyState struct {
	ApiKeyArgs
	Secret             string  `pulumi:"secret" provider:"secret"`
	Revoked            bool    `pulumi:"revoked"`
	Expired            bool    `pulumi:"expired"`
	Expiration         *string `pulumi:"expiration,optional"`
	LastUsedAt         *string `pulumi:"lastUsedAt,optional"`
	EffectiveType      string  `pulumi:"effectiveType"`
	EffectiveSubject   string  `pulumi:"effectiveSubject"`
	EffectiveCreatedBy *string `pulumi:"effectiveCreatedBy,optional"`
	CreatedAt          string  `pulumi:"createdAt"`
	UpdatedAt          string  `pulumi:"updatedAt"`
}

func (r *ApiKey) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk API key for machine-to-machine authentication. The secret value is available only from the output of the resource. The name, type and creator of a key cannot be changed, so changing them replaces the key and issues a new secret.")
}

func (a *ApiKeyArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "Name of the API key. Changing this forces a new resource.")
	ann.Describe(&a.Type, "Type of the API key. When unset Clerk applies its default, reported in effectiveType. Changing this forces a new resource.")
	ann.Describe(&a.Subject, "ID of the user, organization, machine or SCIM subject the key belongs to. Clerk requires a subject when the key is created. The effective value is in effectiveSubject.")
	ann.Describe(&a.Description, "Description of the API key.")
	ann.Describe(&a.Claims, "JSON object of custom claims. Key order and whitespace do not matter: the value is normalised, so reordering keys is not a change.")
	ann.Describe(&a.Scopes, "Scopes granted to the API key.")
	ann.Describe(&a.CreatedBy, "ID of the user that created the key. Can only be set at creation, so changing it forces a new resource.")
	ann.Describe(&a.SecondsUntilExpiration, "Number of seconds until the key expires. Write-only: Clerk reports an expiration timestamp instead, in expiration.")
}

func (s *ApiKeyState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.Secret, "The API key secret. Secret. It is read when the key is created or imported.")
	ann.Describe(&s.Revoked, "Whether the key has been revoked.")
	ann.Describe(&s.Expired, "Whether the key has expired.")
	ann.Describe(&s.Expiration, "When the key expires (RFC 3339), if it has an expiration.")
	ann.Describe(&s.LastUsedAt, "When the key was last used (RFC 3339), if it has been used.")
	ann.Describe(&s.EffectiveType, "The key type Clerk is using, including its default.")
	ann.Describe(&s.EffectiveSubject, "The subject Clerk reports for the key.")
	ann.Describe(&s.EffectiveCreatedBy, "The creator Clerk reports for the key, if any.")
	ann.Describe(&s.CreatedAt, "When the key was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the key was last updated (RFC 3339).")
}

var _ infer.CustomResource[ApiKeyArgs, ApiKeyState] = (*ApiKey)(nil)
var _ infer.CustomCheck[ApiKeyArgs] = (*ApiKey)(nil)
var _ infer.CustomUpdate[ApiKeyArgs, ApiKeyState] = (*ApiKey)(nil)
var _ infer.CustomRead[ApiKeyArgs, ApiKeyState] = (*ApiKey)(nil)
var _ infer.CustomDelete[ApiKeyState] = (*ApiKey)(nil)

// apiKeyJSON is the API shape.
type apiKeyJSON struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Subject     string          `json:"subject"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	Claims      json.RawMessage `json:"claims"`
	Scopes      []string        `json:"scopes"`
	Revoked     bool            `json:"revoked"`
	Expired     bool            `json:"expired"`
	Expiration  *int64          `json:"expiration"`
	CreatedBy   *string         `json:"created_by"`
	LastUsedAt  *int64          `json:"last_used_at"`
	CreatedAt   int64           `json:"created_at"`
	UpdatedAt   int64           `json:"updated_at"`
	// Secret is only present on the create response.
	Secret string `json:"secret"`
}

// claimsPtr is the canonical claims Clerk reports, or nil when it has none.
func (j apiKeyJSON) claimsPtr() *string {
	t := bytes.TrimSpace(j.Claims)
	if len(t) == 0 || string(t) == "null" {
		return nil
	}
	c, err := canonicalizeJSON(string(t))
	if err != nil {
		c = string(t)
	}
	return &c
}

func millisPtrToRFC3339(ms *int64) *string {
	if ms == nil {
		return nil
	}
	return strPtr(millisToRFC3339(*ms))
}

func (j apiKeyJSON) outputs(in ApiKeyArgs, secret string) ApiKeyState {
	return ApiKeyState{
		ApiKeyArgs:         in,
		Secret:             secret,
		Revoked:            j.Revoked,
		Expired:            j.Expired,
		Expiration:         millisPtrToRFC3339(j.Expiration),
		LastUsedAt:         millisPtrToRFC3339(j.LastUsedAt),
		EffectiveType:      j.Type,
		EffectiveSubject:   j.Subject,
		EffectiveCreatedBy: j.CreatedBy,
		CreatedAt:          millisToRFC3339(j.CreatedAt),
		UpdatedAt:          millisToRFC3339(j.UpdatedAt),
	}
}

func apiKeyBody(in ApiKeyArgs, create bool) map[string]any {
	body := map[string]any{}
	if create {
		body["name"] = in.Name
		if in.Type != nil {
			body["type"] = *in.Type
		}
		if in.CreatedBy != nil {
			body["created_by"] = *in.CreatedBy
		}
	}
	if in.Subject != nil {
		body["subject"] = *in.Subject
	}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	if in.Claims != nil {
		body["claims"] = json.RawMessage(*in.Claims)
	}
	if in.Scopes != nil {
		body["scopes"] = in.Scopes
	}
	if in.SecondsUntilExpiration != nil {
		body["seconds_until_expiration"] = *in.SecondsUntilExpiration
	}
	return body
}

// Check normalises the claims so that equivalent JSON documents are equal.
func (r *ApiKey) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ApiKeyArgs], error) {
	args, failures, err := infer.DefaultCheck[ApiKeyArgs](ctx, req.NewInputs)
	if err != nil || len(failures) > 0 {
		return infer.CheckResponse[ApiKeyArgs]{Inputs: args, Failures: failures}, err
	}
	if args.Claims != nil {
		canon, cerr := canonicalizeJSONObject(*args.Claims)
		if cerr != nil {
			failures = append(failures, pgp.CheckFailure{Property: "claims", Reason: fmt.Sprintf("claims must be a valid JSON object: %v", cerr)})
		} else {
			args.Claims = &canon
		}
	}
	return infer.CheckResponse[ApiKeyArgs]{Inputs: args, Failures: failures}, nil
}

func getApiKey(ctx context.Context, c *client, id string) (*apiKeyJSON, error) {
	var out apiKeyJSON
	if err := c.do(ctx, http.MethodGet, "/api_keys/"+id, nil, &out); err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func fetchApiKeySecret(ctx context.Context, c *client, id string) (string, error) {
	var out struct {
		Secret string `json:"secret"`
	}
	if err := c.do(ctx, http.MethodGet, "/api_keys/"+id+"/secret", nil, &out); err != nil {
		return "", err
	}
	return out.Secret, nil
}

func (r *ApiKey) Create(ctx context.Context, req infer.CreateRequest[ApiKeyArgs]) (infer.CreateResponse[ApiKeyState], error) {
	if req.DryRun {
		return infer.CreateResponse[ApiKeyState]{Output: ApiKeyState{ApiKeyArgs: req.Inputs}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[ApiKeyState]{}, err
	}
	var out apiKeyJSON
	if err := c.do(ctx, http.MethodPost, "/api_keys", apiKeyBody(req.Inputs, true), &out); err != nil {
		return infer.CreateResponse[ApiKeyState]{}, fmt.Errorf("clerk: create API key %q: %w", req.Inputs.Name, err)
	}
	secret := out.Secret
	if secret == "" {
		// The key exists; recover the secret or report a partial create so it is not leaked.
		var serr error
		if secret, serr = fetchApiKeySecret(ctx, c, out.ID); serr != nil {
			return infer.CreateResponse[ApiKeyState]{ID: out.ID, Output: out.outputs(req.Inputs, "")},
				infer.ResourceInitFailedError{Reasons: []string{fmt.Sprintf("API key %s was created but its secret could not be read: %v", out.ID, serr)}}
		}
	}
	return infer.CreateResponse[ApiKeyState]{ID: out.ID, Output: out.outputs(req.Inputs, secret)}, nil
}

func (r *ApiKey) Read(ctx context.Context, req infer.ReadRequest[ApiKeyArgs, ApiKeyState]) (infer.ReadResponse[ApiKeyArgs, ApiKeyState], error) {
	empty := infer.ReadResponse[ApiKeyArgs, ApiKeyState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := getApiKey(ctx, c, req.ID)
	if err != nil {
		return empty, fmt.Errorf("clerk: read API key %q: %w", req.ID, err)
	}
	if got == nil {
		if err := notFoundOnImport("api key", req.ID, req.Inputs, req.State); err != nil {
			return empty, err
		}
		return empty, nil
	}
	in := req.Inputs
	imported := in.Name == ""
	in.Name = got.Name
	// Optional inputs the program left unset stay unset; Clerk's value is in
	// the effective* outputs. On import adopt everything Clerk reports.
	if imported || in.Type != nil {
		in.Type = strPtr(got.Type)
	}
	if imported || in.Subject != nil {
		in.Subject = strPtr(got.Subject)
	}
	if imported || in.Description != nil {
		in.Description = got.Description
	}
	if imported || in.Claims != nil {
		in.Claims = got.claimsPtr()
	}
	if imported || in.Scopes != nil {
		if len(got.Scopes) > 0 {
			in.Scopes = got.Scopes
		} else if imported {
			in.Scopes = nil
		} else {
			in.Scopes = []string{}
		}
	}
	if imported || in.CreatedBy != nil {
		in.CreatedBy = got.CreatedBy
	}
	// seconds_until_expiration is write-only (Clerk reports a timestamp), so
	// it stays as the program set it.
	secret := req.State.Secret
	if secret == "" {
		if secret, err = fetchApiKeySecret(ctx, c, req.ID); err != nil {
			return empty, fmt.Errorf("clerk: read secret of API key %q: %w", req.ID, err)
		}
	}
	return infer.ReadResponse[ApiKeyArgs, ApiKeyState]{ID: got.ID, Inputs: in, State: got.outputs(in, secret)}, nil
}

func (r *ApiKey) Update(ctx context.Context, req infer.UpdateRequest[ApiKeyArgs, ApiKeyState]) (infer.UpdateResponse[ApiKeyState], error) {
	if req.DryRun {
		st := req.State
		st.ApiKeyArgs = req.Inputs
		return infer.UpdateResponse[ApiKeyState]{Output: st}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[ApiKeyState]{}, err
	}
	var out apiKeyJSON
	if err := c.do(ctx, http.MethodPatch, "/api_keys/"+req.ID, apiKeyBody(req.Inputs, false), &out); err != nil {
		return infer.UpdateResponse[ApiKeyState]{}, fmt.Errorf("clerk: update API key %q: %w", req.ID, err)
	}
	// The secret is not returned by update: keep the one we hold.
	return infer.UpdateResponse[ApiKeyState]{Output: out.outputs(req.Inputs, req.State.Secret)}, nil
}

func (r *ApiKey) Delete(ctx context.Context, req infer.DeleteRequest[ApiKeyState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/api_keys/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete API key %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetApiKey looks up an API key by ID, or by name within a subject.
type GetApiKey struct{}

// GetApiKeyArgs selects the key: set apiKeyId, or set name together with
// subject (Clerk lists API keys per subject).
type GetApiKeyArgs struct {
	ID      *string `pulumi:"apiKeyId,optional"`
	Name    *string `pulumi:"name,optional"`
	Subject *string `pulumi:"subject,optional"`
}

// GetApiKeyResult is the looked-up key. It never includes the secret.
type GetApiKeyResult struct {
	ID          string   `pulumi:"apiKeyId"`
	Name        string   `pulumi:"name"`
	Type        string   `pulumi:"type"`
	Subject     string   `pulumi:"subject"`
	Description *string  `pulumi:"description,optional"`
	Claims      *string  `pulumi:"claims,optional"`
	Scopes      []string `pulumi:"scopes"`
	Revoked     bool     `pulumi:"revoked"`
	Expired     bool     `pulumi:"expired"`
	Expiration  *string  `pulumi:"expiration,optional"`
	CreatedBy   *string  `pulumi:"createdBy,optional"`
	LastUsedAt  *string  `pulumi:"lastUsedAt,optional"`
	CreatedAt   string   `pulumi:"createdAt"`
	UpdatedAt   string   `pulumi:"updatedAt"`
}

func (f *GetApiKey) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk API key by ID, or by exact name within a subject. The secret is not returned.")
}

func (a *GetApiKeyArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The API key ID. Set either apiKeyId, or name together with subject.")
	ann.Describe(&a.Name, "The exact API key name. Requires subject.")
	ann.Describe(&a.Subject, "The subject (user, organization, machine or SCIM ID) whose keys to search by name.")
}

func (f *GetApiKey) Invoke(ctx context.Context, req infer.FunctionRequest[GetApiKeyArgs]) (infer.FunctionResponse[GetApiKeyResult], error) {
	out := infer.FunctionResponse[GetApiKeyResult]{}
	id, name, subject := derefStr(req.Input.ID), derefStr(req.Input.Name), derefStr(req.Input.Subject)
	if (id == "") == (name == "") || (id != "" && subject != "") || (name != "" && subject == "") {
		return out, fmt.Errorf("clerk: getApiKey needs either apiKeyId, or name together with subject")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	var got *apiKeyJSON
	if id != "" {
		got, err = getApiKey(ctx, c, id)
	} else {
		var items []apiKeyJSON
		items, err = listAll[apiKeyJSON](ctx, c, "/api_keys", url.Values{"subject": {subject}})
		var matches []apiKeyJSON
		for i := range items {
			if items[i].Name == name {
				matches = append(matches, items[i])
			}
		}
		if len(matches) > 1 {
			return out, fmt.Errorf("clerk: getApiKey: %d API keys named %q exist for subject %s; look it up by apiKeyId", len(matches), name, subject)
		}
		if len(matches) == 1 {
			got = &matches[0]
		}
	}
	what := id + name
	if err != nil {
		return out, lookupErr("API key", what, err)
	}
	if got == nil {
		return out, lookupErr("API key", fmt.Sprintf("%q", what), &apiError{Status: http.StatusNotFound})
	}
	scopes := got.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	out.Output = GetApiKeyResult{ID: got.ID, Name: got.Name, Type: got.Type, Subject: got.Subject, Description: got.Description,
		Claims: got.claimsPtr(), Scopes: scopes, Revoked: got.Revoked, Expired: got.Expired, Expiration: millisPtrToRFC3339(got.Expiration),
		CreatedBy: got.CreatedBy, LastUsedAt: millisPtrToRFC3339(got.LastUsedAt),
		CreatedAt: millisToRFC3339(got.CreatedAt), UpdatedAt: millisToRFC3339(got.UpdatedAt)}
	return out, nil
}
