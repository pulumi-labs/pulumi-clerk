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
	"net/http"
	"testing"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		// The client secret is returned by create only; reads never include it.
		derive := func(o fakeObj, body map[string]any) error {
			id := o.str("id")
			o["client_id"] = "client_" + id
			o["discovery_url"] = "https://clerk.example.com/.well-known/openid-configuration"
			o["authorize_url"] = "https://clerk.example.com/oauth/authorize"
			o["token_fetch_url"] = "https://clerk.example.com/oauth/token"
			o["user_info_url"] = "https://clerk.example.com/oauth/userinfo"
			if _, ok := o["consent_screen_enabled"]; !ok {
				o["consent_screen_enabled"] = false
			}
			// Clerk reorders scopes and adds implicit ones.
			o["scopes"] = o.str("scopes") + " offline_access"
			return nil
		}
		f.collection(&fakeCollection{
			Path: "/oauth_applications", Prefix: "oa", Object: "oauth_application", Patch: true,
			OnCreate: func(o fakeObj, body map[string]any) error {
				if err := derive(o, body); err != nil {
					return err
				}
				o["client_secret"] = "cs_secret_" + o.str("id")
				return nil
			},
			OnUpdate: func(o fakeObj, body map[string]any) error {
				if _, ok := body["scopes"]; ok {
					o["scopes"] = o.str("scopes") + " offline_access"
				}
				return nil
			},
		})
		f.mux.HandleFunc("GET /oauth_applications/{id}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			o, ok := f.collections["/oauth_applications"].Objs[r.PathValue("id")]
			if !ok {
				notFound(w, "oauth application")
				return
			}
			cp := fakeObj{}
			for k, v := range o {
				if k != "client_secret" {
					cp[k] = v
				}
			}
			writeJSON(w, http.StatusOK, cp)
		})
	})
}

func oauthInputs() property.Map {
	return pm(map[string]property.Value{
		"name": str("My App"), "callbackUrl": str("https://app.example.com/cb"),
		"scopes": str("profile email"), "public": boolean(false),
	})
}

func TestOauthApplication_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("OauthApplication", "o")

	inputs := oauthInputs()
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "client_"+created.ID, getStr(t, created.Properties, "clientId"))
	require.Equal(t, "cs_secret_"+created.ID, getStr(t, created.Properties, "clientSecret"))
	require.Equal(t, "profile email", getStr(t, created.Properties, "scopes"))
	require.Equal(t, "profile email offline_access", getStr(t, created.Properties, "effectiveScopes"))
	requireSecretInSchema(t, s, "OauthApplication", "clientSecret")

	// Idempotent diff.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Refresh keeps the program's scopes (Clerk added offline_access) and the secret.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "profile email", getStr(t, read.Inputs, "scopes"))
	require.Equal(t, "cs_secret_"+created.ID, getStr(t, read.Properties, "clientSecret"))
	_, hasConsent := read.Inputs.GetOk("consentScreenEnabled")
	require.False(t, hasConsent)
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: read.Properties, Inputs: inputs, OldInputs: read.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Scope drift behind Pulumi's back is detected.
	f.mu.Lock()
	f.collections["/oauth_applications"].Objs[created.ID]["scopes"] = "profile offline_access"
	f.mu.Unlock()
	drift, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "profile offline_access", getStr(t, drift.Inputs, "scopes"))
	f.mu.Lock()
	f.collections["/oauth_applications"].Objs[created.ID]["scopes"] = "profile email offline_access"
	f.mu.Unlock()

	// Update in place.
	next := pm(map[string]property.Value{
		"name": str("My App 2"), "callbackUrl": str("https://app.example.com/cb2"),
		"scopes": str("profile"), "public": boolean(false), "consentScreenEnabled": boolean(true),
	})
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: inputs})
	require.NoError(t, err)
	require.True(t, d.HasChanges)
	require.NotEqual(t, pgp.UpdateReplace, d.DetailedDiff["name"].Kind)
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "My App 2", getStr(t, upd.Properties, "name"))
	require.Equal(t, "cs_secret_"+created.ID, getStr(t, upd.Properties, "clientSecret"), "update must keep the creation-time secret")

	// Changing public replaces.
	repl := pm(map[string]property.Value{
		"name": str("My App 2"), "callbackUrl": str("https://app.example.com/cb2"),
		"scopes": str("profile"), "public": boolean(true), "consentScreenEnabled": boolean(true),
	})
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: upd.Properties, Inputs: repl, OldInputs: next})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["public"].Kind)

	// Import by ID alone adopts what Clerk reports and cannot know the secret.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "My App 2", getStr(t, imp.Inputs, "name"))
	require.Equal(t, "profile offline_access", getStr(t, imp.Inputs, "scopes"))
	require.Empty(t, getStr(t, imp.Properties, "clientSecret"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)
	_, adopted := imp.Inputs.GetOk("consentScreenEnabled")
	require.False(t, adopted, "import must not adopt the server default (#1563)")
	// Reordered or partial scopes are not a change (Clerk reorders and adds
	// implicit ones); a scope Clerk does not have is.
	for _, scopes := range []string{"offline_access profile", "profile"} {
		prog := with(imp.Inputs, map[string]property.Value{"scopes": str(scopes)})
		d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: prog, OldInputs: imp.Inputs})
		require.NoError(t, err)
		requireNoChanges(t, d)
	}
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, OldInputs: imp.Inputs,
		Inputs: with(imp.Inputs, map[string]property.Value{"scopes": str("profile email")})})
	require.NoError(t, err)
	require.True(t, d.HasChanges)

	// Lookup function by ID, client ID and name; never leaks the secret.
	out, err := invoke(t, s, "getOauthApplication", map[string]property.Value{"oauthApplicationId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "My App 2", getStr(t, out, "name"))
	_, leaked := out.GetOk("clientSecret")
	require.False(t, leaked)
	out, err = invoke(t, s, "getOauthApplication", map[string]property.Value{"clientId": str("client_" + created.ID)})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "oauthApplicationId"))
	out, err = invoke(t, s, "getOauthApplication", map[string]property.Value{"name": str("My App 2")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "oauthApplicationId"))
	_, err = invoke(t, s, "getOauthApplication", map[string]property.Value{"name": str("nobody")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getOauthApplication", map[string]property.Value{"oauthApplicationId": str("oa_missing")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getOauthApplication", map[string]property.Value{"name": str("a"), "clientId": str("b")})
	require.ErrorContains(t, err, "exactly one of")

	// Deleted outside Pulumi.
	f.collections["/oauth_applications"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: next})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestOauthApplication_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("OauthApplication", "o")
	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: oauthInputs(), DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST"))
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: oauthInputs()})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/oauth_applications"].Objs)
}

func TestOauthApplication_ServerRejectionIsSurfaced(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	f.collections["/oauth_applications"].OnCreate = func(fakeObj, map[string]any) error {
		return oauthTestErr("callback_url must use https")
	}
	_, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("OauthApplication", "o"), Properties: oauthInputs()})
	require.ErrorContains(t, err, "callback_url must use https")
}

type oauthTestErr string

func (e oauthTestErr) Error() string { return string(e) }

// withProps returns m with the given properties set.
func with(m property.Map, set map[string]property.Value) property.Map {
	for k, v := range set {
		m = m.Set(k, v)
	}
	return m
}
