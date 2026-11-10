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

// canonicalizeJSON re-encodes a JSON document with sorted object keys and
// compact whitespace, preserving number literals, so that two documents that
// differ only in key order or formatting compare equal as strings.
func canonicalizeJSON(s string) (string, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	if dec.More() {
		return "", fmt.Errorf("unexpected data after the JSON document")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// canonicalizeJSONObject is canonicalizeJSON for a document that must be a
// JSON object (claims).
func canonicalizeJSONObject(s string) (string, error) {
	c, err := canonicalizeJSON(s)
	if err != nil {
		return "", err
	}
	if len(c) == 0 || c[0] != '{' {
		return "", fmt.Errorf("must be a JSON object")
	}
	return c, nil
}

// JwtTemplate is a custom JWT template used to mint session tokens for
// third-party integrations.
type JwtTemplate struct{}

// JwtTemplateArgs are the inputs of a JwtTemplate.
type JwtTemplateArgs struct {
	// Name is the template name, used as the token template identifier.
	Name string `pulumi:"name"`
	// Claims is the JSON object of custom claims.
	Claims string `pulumi:"claims"`
	// Lifetime is the token lifetime in seconds.
	Lifetime *int `pulumi:"lifetime,optional"`
	// AllowedClockSkew is the allowed clock skew in seconds.
	AllowedClockSkew *int `pulumi:"allowedClockSkew,optional"`
	// CustomSigningKey makes the template use SigningKey instead of the instance key.
	CustomSigningKey *bool `pulumi:"customSigningKey,optional"`
	// SigningKey is the custom signing key; Clerk never returns it.
	SigningKey *string `pulumi:"signingKey,optional" provider:"secret"`
	// SigningAlgorithm is the algorithm used to sign tokens, for example RS256 or HS256.
	SigningAlgorithm *string `pulumi:"signingAlgorithm,optional"`
}

// JwtTemplateState is the stored state of a JwtTemplate.
type JwtTemplateState struct {
	JwtTemplateArgs
	EffectiveLifetime         int    `pulumi:"effectiveLifetime"`
	EffectiveAllowedClockSkew int    `pulumi:"effectiveAllowedClockSkew"`
	EffectiveCustomSigningKey bool   `pulumi:"effectiveCustomSigningKey"`
	EffectiveSigningAlgorithm string `pulumi:"effectiveSigningAlgorithm"`
	CreatedAt                 string `pulumi:"createdAt"`
	UpdatedAt                 string `pulumi:"updatedAt"`
}

func (r *JwtTemplate) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk JWT template: a named set of custom claims, lifetime and signing settings used to mint tokens for third-party integrations.")
}

func (a *JwtTemplateArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "Name of the JWT template.")
	ann.Describe(&a.Claims, "JSON object of custom claims for the token. Key order and whitespace do not matter: the value is normalised, so reordering keys is not a change.")
	ann.Describe(&a.Lifetime, "Token lifetime in seconds. When unset Clerk applies its default, reported in effectiveLifetime.")
	ann.Describe(&a.AllowedClockSkew, "Allowed clock skew in seconds. When unset Clerk applies its default, reported in effectiveAllowedClockSkew.")
	ann.Describe(&a.CustomSigningKey, "Whether to sign tokens with signingKey instead of the instance signing key.")
	ann.Describe(&a.SigningKey, "Custom signing key. Write-only: Clerk never returns it, so it is kept from the program.")
	ann.Describe(&a.SigningAlgorithm, "Signing algorithm, for example RS256 or HS256. When unset Clerk applies its default, reported in effectiveSigningAlgorithm.")
}

func (s *JwtTemplateState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.EffectiveLifetime, "The token lifetime in seconds that Clerk is using, including its default.")
	ann.Describe(&s.EffectiveAllowedClockSkew, "The allowed clock skew in seconds that Clerk is using, including its default.")
	ann.Describe(&s.EffectiveCustomSigningKey, "Whether Clerk signs with a custom signing key.")
	ann.Describe(&s.EffectiveSigningAlgorithm, "The signing algorithm Clerk is using, including its default.")
	ann.Describe(&s.CreatedAt, "When the template was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the template was last updated (RFC 3339).")
}

var _ infer.CustomResource[JwtTemplateArgs, JwtTemplateState] = (*JwtTemplate)(nil)
var _ infer.CustomCheck[JwtTemplateArgs] = (*JwtTemplate)(nil)
var _ infer.CustomUpdate[JwtTemplateArgs, JwtTemplateState] = (*JwtTemplate)(nil)
var _ infer.CustomRead[JwtTemplateArgs, JwtTemplateState] = (*JwtTemplate)(nil)
var _ infer.CustomDelete[JwtTemplateState] = (*JwtTemplate)(nil)

// jwtTemplateJSON is the API shape.
type jwtTemplateJSON struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Claims           json.RawMessage `json:"claims"`
	Lifetime         int             `json:"lifetime"`
	AllowedClockSkew int             `json:"allowed_clock_skew"`
	CustomSigningKey bool            `json:"custom_signing_key"`
	SigningAlgorithm string          `json:"signing_algorithm"`
	CreatedAt        int64           `json:"created_at"`
	UpdatedAt        int64           `json:"updated_at"`
}

// claimsString is the canonical form of the claims Clerk reports.
func (j jwtTemplateJSON) claimsString() string {
	if len(bytes.TrimSpace(j.Claims)) == 0 {
		return "{}"
	}
	if c, err := canonicalizeJSON(string(j.Claims)); err == nil {
		return c
	}
	return string(j.Claims)
}

// outputs fills the server-side fields of a state around the given inputs.
func (j jwtTemplateJSON) outputs(in JwtTemplateArgs) JwtTemplateState {
	return JwtTemplateState{
		JwtTemplateArgs:           in,
		EffectiveLifetime:         j.Lifetime,
		EffectiveAllowedClockSkew: j.AllowedClockSkew,
		EffectiveCustomSigningKey: j.CustomSigningKey,
		EffectiveSigningAlgorithm: j.SigningAlgorithm,
		CreatedAt:                 millisToRFC3339(j.CreatedAt),
		UpdatedAt:                 millisToRFC3339(j.UpdatedAt),
	}
}

func jwtTemplateBody(in JwtTemplateArgs) map[string]any {
	body := map[string]any{"name": in.Name, "claims": json.RawMessage(in.Claims)}
	if in.Lifetime != nil {
		body["lifetime"] = *in.Lifetime
	}
	if in.AllowedClockSkew != nil {
		body["allowed_clock_skew"] = *in.AllowedClockSkew
	}
	if in.CustomSigningKey != nil {
		body["custom_signing_key"] = *in.CustomSigningKey
	}
	if in.SigningKey != nil {
		body["signing_key"] = *in.SigningKey
	}
	if in.SigningAlgorithm != nil {
		body["signing_algorithm"] = *in.SigningAlgorithm
	}
	return body
}

// Check normalises the claims so that equivalent JSON documents are equal.
func (r *JwtTemplate) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[JwtTemplateArgs], error) {
	args, failures, err := infer.DefaultCheck[JwtTemplateArgs](ctx, req.NewInputs)
	if err != nil || len(failures) > 0 {
		return infer.CheckResponse[JwtTemplateArgs]{Inputs: args, Failures: failures}, err
	}
	canon, cerr := canonicalizeJSONObject(args.Claims)
	if cerr != nil {
		failures = append(failures, pgp.CheckFailure{Property: "claims", Reason: fmt.Sprintf("claims must be a valid JSON object: %v", cerr)})
	} else {
		args.Claims = canon
	}
	return infer.CheckResponse[JwtTemplateArgs]{Inputs: args, Failures: failures}, nil
}

func getJwtTemplate(ctx context.Context, c *client, id string) (*jwtTemplateJSON, error) {
	var out jwtTemplateJSON
	if err := c.do(ctx, http.MethodGet, "/jwt_templates/"+id, nil, &out); err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func (r *JwtTemplate) Create(ctx context.Context, req infer.CreateRequest[JwtTemplateArgs]) (infer.CreateResponse[JwtTemplateState], error) {
	if req.DryRun {
		return infer.CreateResponse[JwtTemplateState]{Output: JwtTemplateState{JwtTemplateArgs: req.Inputs}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[JwtTemplateState]{}, err
	}
	var out jwtTemplateJSON
	if err := c.do(ctx, http.MethodPost, "/jwt_templates", jwtTemplateBody(req.Inputs), &out); err != nil {
		return infer.CreateResponse[JwtTemplateState]{}, fmt.Errorf("clerk: create JWT template %q: %w", req.Inputs.Name, err)
	}
	return infer.CreateResponse[JwtTemplateState]{ID: out.ID, Output: out.outputs(req.Inputs)}, nil
}

func (r *JwtTemplate) Read(ctx context.Context, req infer.ReadRequest[JwtTemplateArgs, JwtTemplateState]) (infer.ReadResponse[JwtTemplateArgs, JwtTemplateState], error) {
	empty := infer.ReadResponse[JwtTemplateArgs, JwtTemplateState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := getJwtTemplate(ctx, c, req.ID)
	if err != nil {
		return empty, fmt.Errorf("clerk: read JWT template %q: %w", req.ID, err)
	}
	if got == nil {
		if err := notFoundOnImport("jwt template", req.ID, req.Inputs, req.State); err != nil {
			return empty, err
		}
		return empty, nil
	}
	in := req.Inputs
	imported := in.Name == ""
	in.Name, in.Claims = got.Name, got.claimsString()
	// Optional inputs the program left unset stay unset; the server's value
	// is in the effective* outputs. An import has no program yet, and Clerk
	// fills in defaults for these (#1563), so none are adopted: a program that
	// declares one later sees an in-place update at most.
	if !imported && in.Lifetime != nil {
		v := got.Lifetime
		in.Lifetime = &v
	}
	if !imported && in.AllowedClockSkew != nil {
		v := got.AllowedClockSkew
		in.AllowedClockSkew = &v
	}
	if !imported && in.CustomSigningKey != nil {
		v := got.CustomSigningKey
		in.CustomSigningKey = &v
	}
	if !imported && in.SigningAlgorithm != nil {
		v := got.SigningAlgorithm
		in.SigningAlgorithm = &v
	}
	// The signing key is write-only: keep what we have.
	if in.SigningKey == nil {
		in.SigningKey = req.State.SigningKey
	}
	st := got.outputs(in)
	return infer.ReadResponse[JwtTemplateArgs, JwtTemplateState]{ID: got.ID, Inputs: in, State: st}, nil
}

func (r *JwtTemplate) Update(ctx context.Context, req infer.UpdateRequest[JwtTemplateArgs, JwtTemplateState]) (infer.UpdateResponse[JwtTemplateState], error) {
	if req.DryRun {
		st := req.State
		st.JwtTemplateArgs = req.Inputs
		return infer.UpdateResponse[JwtTemplateState]{Output: st}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[JwtTemplateState]{}, err
	}
	var out jwtTemplateJSON
	if err := c.do(ctx, http.MethodPatch, "/jwt_templates/"+req.ID, jwtTemplateBody(req.Inputs), &out); err != nil {
		return infer.UpdateResponse[JwtTemplateState]{}, fmt.Errorf("clerk: update JWT template %q: %w", req.ID, err)
	}
	return infer.UpdateResponse[JwtTemplateState]{Output: out.outputs(req.Inputs)}, nil
}

func (r *JwtTemplate) Delete(ctx context.Context, req infer.DeleteRequest[JwtTemplateState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/jwt_templates/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete JWT template %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetJwtTemplate looks up a JWT template by ID or name.
type GetJwtTemplate struct{}

// GetJwtTemplateArgs selects the template; set exactly one of the fields.
type GetJwtTemplateArgs struct {
	ID   *string `pulumi:"jwtTemplateId,optional"`
	Name *string `pulumi:"name,optional"`
}

// GetJwtTemplateResult is the looked-up template.
type GetJwtTemplateResult struct {
	ID               string `pulumi:"jwtTemplateId"`
	Name             string `pulumi:"name"`
	Claims           string `pulumi:"claims"`
	Lifetime         int    `pulumi:"lifetime"`
	AllowedClockSkew int    `pulumi:"allowedClockSkew"`
	CustomSigningKey bool   `pulumi:"customSigningKey"`
	SigningAlgorithm string `pulumi:"signingAlgorithm"`
	CreatedAt        string `pulumi:"createdAt"`
	UpdatedAt        string `pulumi:"updatedAt"`
}

func (f *GetJwtTemplate) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk JWT template by ID or by name.")
}

func (a *GetJwtTemplateArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The JWT template ID. Set either jwtTemplateId or name.")
	ann.Describe(&a.Name, "The JWT template name. Set either jwtTemplateId or name.")
}

func (f *GetJwtTemplate) Invoke(ctx context.Context, req infer.FunctionRequest[GetJwtTemplateArgs]) (infer.FunctionResponse[GetJwtTemplateResult], error) {
	out := infer.FunctionResponse[GetJwtTemplateResult]{}
	id, name := derefStr(req.Input.ID), derefStr(req.Input.Name)
	if (id == "") == (name == "") {
		return out, fmt.Errorf("clerk: getJwtTemplate needs exactly one of jwtTemplateId or name")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	var got *jwtTemplateJSON
	if id != "" {
		got, err = getJwtTemplate(ctx, c, id)
	} else {
		var items []jwtTemplateJSON
		items, err = listAll[jwtTemplateJSON](ctx, c, "/jwt_templates", url.Values{"paginated": {"true"}})
		for i := range items {
			if items[i].Name == name {
				got = &items[i]
				break
			}
		}
	}
	if err != nil {
		return out, lookupErr("JWT template", id+name, err)
	}
	if got == nil {
		return out, lookupErr("JWT template", fmt.Sprintf("%q", id+name), &apiError{Status: http.StatusNotFound})
	}
	st := got.outputs(JwtTemplateArgs{})
	out.Output = GetJwtTemplateResult{ID: got.ID, Name: got.Name, Claims: got.claimsString(), Lifetime: got.Lifetime,
		AllowedClockSkew: got.AllowedClockSkew, CustomSigningKey: got.CustomSigningKey, SigningAlgorithm: got.SigningAlgorithm,
		CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt}
	return out, nil
}
