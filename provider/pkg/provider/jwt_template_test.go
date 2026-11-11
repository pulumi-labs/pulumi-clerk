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
	"testing"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		fill := func(o fakeObj, body map[string]any) error {
			// Clerk defaults the optional settings and never returns the signing key.
			if _, ok := o["lifetime"]; !ok {
				o["lifetime"] = float64(60)
			}
			if _, ok := o["allowed_clock_skew"]; !ok {
				o["allowed_clock_skew"] = float64(5)
			}
			if _, ok := o["signing_algorithm"]; !ok {
				o["signing_algorithm"] = "RS256"
			}
			if _, ok := o["custom_signing_key"]; !ok {
				o["custom_signing_key"] = false
			}
			delete(o, "signing_key")
			return nil
		}
		f.collection(&fakeCollection{
			Path: "/jwt_templates", Prefix: "jwt", Object: "jwt_template", ByID: true, Patch: true,
			OnCreate: fill, OnUpdate: fill,
		})
	})
}

func jwtInputs(claims string) property.Map {
	return pm(map[string]property.Value{"name": str("hasura"), "claims": str(claims)})
}

func TestJwtTemplate_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("JwtTemplate", "j")

	inputs := jwtInputs(`{"b":2,"a":{"y":1,"x":[1,2]}}`)
	chk, err := s.Check(pgp.CheckRequest{Urn: urn, Inputs: inputs})
	require.NoError(t, err)
	require.Empty(t, chk.Failures)
	checked := chk.Inputs
	// Claims are normalised to sorted keys.
	require.Equal(t, `{"a":{"x":[1,2],"y":1},"b":2}`, getStr(t, checked, "claims"))

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: checked})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	// Server defaults are exposed as outputs without polluting the inputs.
	require.Equal(t, float64(60), created.Properties.Get("effectiveLifetime").AsNumber())
	require.Equal(t, "RS256", getStr(t, created.Properties, "effectiveSigningAlgorithm"))
	_, hasLifetime := created.Properties.GetOk("lifetime")
	require.False(t, hasLifetime)

	// Idempotent diff, also when the program reorders the claims.
	reordered, err := s.Check(pgp.CheckRequest{Urn: urn, State: checked, Inputs: jwtInputs(`{"a":{"x":[1,2],"y":1},"b":2}`)})
	require.NoError(t, err)
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: reordered.Inputs, OldInputs: checked})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Refresh keeps unset optionals unset and does not blank anything.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: checked})
	require.NoError(t, err)
	_, hasLifetime = read.Inputs.GetOk("lifetime")
	require.False(t, hasLifetime)
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: read.Properties, Inputs: checked, OldInputs: read.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Update in place.
	next := pm(map[string]property.Value{"name": str("hasura"), "claims": str(`{"a":1}`), "lifetime": num(120)})
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: checked})
	require.NoError(t, err)
	require.True(t, d.HasChanges)
	require.Equal(t, pgp.Add, d.DetailedDiff["lifetime"].Kind)
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: checked})
	require.NoError(t, err)
	require.Equal(t, float64(120), upd.Properties.Get("effectiveLifetime").AsNumber())
	require.Equal(t, float64(120), f.collections["/jwt_templates"].Objs[created.ID]["lifetime"])

	// Update preview does not call the API.
	before := f.callCount("PATCH")
	_, err = s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: upd.Properties, Inputs: next, OldInputs: next, DryRun: true})
	require.NoError(t, err)
	require.Equal(t, before, f.callCount("PATCH"))

	// Import by ID alone adopts what Clerk reports, then diffs cleanly.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "hasura", getStr(t, imp.Inputs, "name"))
	// A non-default optional (lifetime 120) is adopted so the program that
	// declares it diffs clean (#1801); optionals at Clerk's default stay unset
	// (#1563).
	require.Equal(t, float64(120), imp.Inputs.Get("lifetime").AsNumber(), "import must adopt a non-default lifetime (#1801)")
	for _, k := range []string{"allowedClockSkew", "customSigningKey", "signingAlgorithm"} {
		_, adopted := imp.Inputs.GetOk(k)
		require.False(t, adopted, "import must not adopt %s at its default", k)
	}
	require.Equal(t, float64(120), imp.Properties.Get("effectiveLifetime").AsNumber())
	program := pm(map[string]property.Value{"name": str("hasura"), "claims": imp.Inputs.Get("claims"), "lifetime": num(120)})
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: program, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function.
	out, err := invoke(t, s, "getJwtTemplate", map[string]property.Value{"name": str("hasura")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "jwtTemplateId"))
	require.Equal(t, `{"a":1}`, getStr(t, out, "claims"))
	out, err = invoke(t, s, "getJwtTemplate", map[string]property.Value{"jwtTemplateId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "hasura", getStr(t, out, "name"))
	_, err = invoke(t, s, "getJwtTemplate", map[string]property.Value{"name": str("nope")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getJwtTemplate", map[string]property.Value{"jwtTemplateId": str("jwt_missing")})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi.
	f.collections["/jwt_templates"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: next})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestJwtTemplate_Delete(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("JwtTemplate", "j")
	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: jwtInputs(`{}`), DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST"))
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: jwtInputs(`{}`)})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/jwt_templates"].Objs)
}

// The signing key is a secret, is sent to Clerk, and survives refresh even
// though Clerk never returns it.
func TestJwtTemplate_SigningKeySecret(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("JwtTemplate", "j")
	inputs := pm(map[string]property.Value{
		"name": str("custom"), "claims": str(`{}`), "customSigningKey": boolean(true),
		"signingKey": property.New("PRIVATE-KEY").WithSecret(true), "signingAlgorithm": str("HS256"),
	})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	k, ok := created.Properties.GetOk("signingKey")
	require.True(t, ok)
	require.True(t, k.Secret())
	require.Equal(t, "PRIVATE-KEY", k.AsString())

	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	k, ok = read.Properties.GetOk("signingKey")
	require.True(t, ok)
	require.Equal(t, "PRIVATE-KEY", k.AsString())
	require.True(t, k.Secret())
}

func TestJwtTemplate_InvalidClaims(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	for _, bad := range []string{`not json`, `[1,2]`, `{"a":1} trailing`} {
		chk, err := s.Check(pgp.CheckRequest{Urn: clerkUrn("JwtTemplate", "j"), Inputs: jwtInputs(bad)})
		require.NoError(t, err)
		require.Len(t, chk.Failures, 1, bad)
		require.Equal(t, "claims", chk.Failures[0].Property)
	}
}
