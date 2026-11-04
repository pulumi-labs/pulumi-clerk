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
	"time"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

const apiKeyFailSecretKey = "apikey.secretFailure"

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		secrets := map[string]string{}
		// Reads never include the secret; it is available from /secret.
		fill := func(o fakeObj, body map[string]any) error {
			if _, ok := o["type"]; !ok {
				o["type"] = "api_key"
			}
			o["revoked"], o["expired"] = false, false
			if _, ok := o["scopes"]; !ok {
				o["scopes"] = []any{}
			}
			if _, ok := o["description"]; !ok {
				o["description"] = nil
			}
			if secs, ok := body["seconds_until_expiration"].(float64); ok {
				o["expiration"] = time.Now().Add(time.Duration(secs) * time.Second).UnixMilli()
				delete(o, "seconds_until_expiration")
			}
			return nil
		}
		f.collection(&fakeCollection{
			Path: "/api_keys", Prefix: "ak", Object: "api_key", Patch: true,
			OnCreate: func(o fakeObj, body map[string]any) error {
				if o.str("subject") == "" {
					return apiKeyTestErr("subject is required")
				}
				if err := fill(o, body); err != nil {
					return err
				}
				// Clerk returns the secret on create; the fake keeps it aside and
				// only echoes it when apikey.inline is set, so both the inline
				// path and the secret-endpoint fallback are exercised.
				secret := "ak_live_" + o.str("id")
				secrets[o.str("id")] = secret
				if f.state["apikey.inline"] == true {
					o["secret"] = secret
				}
				return nil
			},
			OnUpdate: func(o fakeObj, body map[string]any) error { return fill(o, body) },
		})
		strip := func(o fakeObj) fakeObj {
			cp := fakeObj{}
			for k, v := range o {
				if k != "secret" {
					cp[k] = v
				}
			}
			return cp
		}
		f.mux.HandleFunc("GET /api_keys/{id}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			o, ok := f.collections["/api_keys"].Objs[r.PathValue("id")]
			if !ok {
				notFound(w, "api key")
				return
			}
			writeJSON(w, http.StatusOK, strip(o))
		})
		f.mux.HandleFunc("GET /api_keys/{id}/secret", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			o, ok := f.collections["/api_keys"].Objs[r.PathValue("id")]
			if !ok {
				notFound(w, "api key")
				return
			}
			if f.state[apiKeyFailSecretKey] == true {
				writeAPIError(w, http.StatusForbidden, "forbidden", "not allowed to read secrets")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"object": "api_key_secret", "secret": secrets[o.str("id")]})
		})
	})
}

type apiKeyTestErr string

func (e apiKeyTestErr) Error() string { return string(e) }

const testSubject = "user_2abcdefghijklmnopqrstuvwxyz"

func apiKeyInputs() property.Map {
	return pm(map[string]property.Value{
		"name": str("ci-key"), "subject": str(testSubject),
		"scopes": strArray("read", "write"), "claims": str(`{"b":1,"a":2}`),
	})
}

func TestApiKey_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("ApiKey", "k")

	chk, err := s.Check(pgp.CheckRequest{Urn: urn, Inputs: apiKeyInputs()})
	require.NoError(t, err)
	require.Empty(t, chk.Failures)
	inputs := chk.Inputs
	require.Equal(t, `{"a":2,"b":1}`, getStr(t, inputs, "claims"))

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "ak_live_"+created.ID, getStr(t, created.Properties, "secret"))
	require.Equal(t, "api_key", getStr(t, created.Properties, "effectiveType"))
	require.Equal(t, testSubject, getStr(t, created.Properties, "effectiveSubject"))
	_, hasType := created.Properties.GetOk("type")
	require.False(t, hasType, "type was not set by the program so it stays out of the inputs")
	requireSecretInSchema(t, s, "ApiKey", "secret")

	// Idempotent diff, including reordered claims.
	re, err := s.Check(pgp.CheckRequest{Urn: urn, State: inputs, Inputs: pm(map[string]property.Value{
		"name": str("ci-key"), "subject": str(testSubject),
		"scopes": strArray("read", "write"), "claims": str(`{ "a": 2, "b": 1 }`),
	})})
	require.NoError(t, err)
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: re.Inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Refresh keeps secret and inputs, does not refetch the secret.
	before := f.callCount("GET /api_keys/" + created.ID + "/secret")
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "ak_live_"+created.ID, getStr(t, read.Properties, "secret"))
	require.Equal(t, before, f.callCount("GET /api_keys/"+created.ID+"/secret"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: read.Properties, Inputs: inputs, OldInputs: read.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Updatable fields change in place and the secret survives.
	next := pm(map[string]property.Value{
		"name": str("ci-key"), "subject": str(testSubject), "description": str("for CI"),
		"scopes": strArray("read"), "claims": str(`{"a":2,"b":1}`), "secondsUntilExpiration": num(3600),
	})
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: inputs})
	require.NoError(t, err)
	require.True(t, d.HasChanges)
	for k, diff := range d.DetailedDiff {
		require.NotEqual(t, pgp.UpdateReplace, diff.Kind, "%s should update in place", k)
		require.NotEqual(t, pgp.AddReplace, diff.Kind, "%s should update in place", k)
	}
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "ak_live_"+created.ID, getStr(t, upd.Properties, "secret"))
	require.Equal(t, "for CI", getStr(t, upd.Properties, "description"))
	require.NotEmpty(t, getStr(t, upd.Properties, "expiration"))

	// Name and type cannot be updated: replace.
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: upd.Properties, OldInputs: next, Inputs: pm(map[string]property.Value{
		"name": str("renamed"), "subject": str(testSubject), "description": str("for CI"),
		"scopes": strArray("read"), "claims": str(`{"a":2,"b":1}`), "secondsUntilExpiration": num(3600),
	})})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["name"].Kind)
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: upd.Properties, OldInputs: next, Inputs: pm(map[string]property.Value{
		"name": str("ci-key"), "type": str("other_type"), "subject": str(testSubject), "description": str("for CI"),
		"scopes": strArray("read"), "claims": str(`{"a":2,"b":1}`), "secondsUntilExpiration": num(3600),
	})})
	require.NoError(t, err)
	require.Contains(t, []pgp.DiffKind{pgp.AddReplace, pgp.UpdateReplace}, d.DetailedDiff["type"].Kind)

	// Import by ID alone recovers the secret and diffs cleanly.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "ci-key", getStr(t, imp.Inputs, "name"))
	require.Equal(t, "ak_live_"+created.ID, getStr(t, imp.Properties, "secret"))
	require.Equal(t, `{"a":2,"b":1}`, getStr(t, imp.Inputs, "claims"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function.
	out, err := invoke(t, s, "getApiKey", map[string]property.Value{"apiKeyId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "ci-key", getStr(t, out, "name"))
	_, leaked := out.GetOk("secret")
	require.False(t, leaked)
	out, err = invoke(t, s, "getApiKey", map[string]property.Value{"name": str("ci-key"), "subject": str(testSubject)})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "apiKeyId"))
	_, err = invoke(t, s, "getApiKey", map[string]property.Value{"name": str("nope"), "subject": str(testSubject)})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getApiKey", map[string]property.Value{"apiKeyId": str("ak_missing")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getApiKey", map[string]property.Value{"name": str("ci-key")})
	require.ErrorContains(t, err, "together with subject")

	// Deleted outside Pulumi.
	f.collections["/api_keys"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: next})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestApiKey_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("ApiKey", "k")
	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: apiKeyInputs(), DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST"))
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: apiKeyInputs()})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/api_keys"].Objs)
}

func TestApiKey_ValidationAndServerErrors(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("ApiKey", "k")

	chk, err := s.Check(pgp.CheckRequest{Urn: urn, Inputs: pm(map[string]property.Value{"name": str("k"), "claims": str("[1]")})})
	require.NoError(t, err)
	require.Len(t, chk.Failures, 1)
	require.Equal(t, "claims", chk.Failures[0].Property)

	// Clerk's own validation is surfaced.
	_, err = s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"name": str("no-subject")})})
	require.ErrorContains(t, err, "subject is required")
}

// A key whose secret cannot be read after creation is reported as a partial
// create, keeping the ID so the key is not leaked.
func TestApiKey_PartialCreate(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	f.state[apiKeyFailSecretKey] = true
	_, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("ApiKey", "k"), Properties: apiKeyInputs()})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to initialize")
	require.Len(t, f.collections["/api_keys"].Objs, 1)
}

// When Clerk returns the secret inline it is used without another call.
func TestApiKey_InlineSecret(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	f.state["apikey.inline"] = true
	created, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("ApiKey", "k"), Properties: apiKeyInputs()})
	require.NoError(t, err)
	require.Equal(t, "ak_live_"+created.ID, getStr(t, created.Properties, "secret"))
	require.Zero(t, f.callCount("GET /api_keys/"+created.ID+"/secret"))
}
