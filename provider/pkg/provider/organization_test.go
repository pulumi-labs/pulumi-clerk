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
	"errors"
	"net/http"
	"strings"
	"testing"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		orgs := f.collection(&fakeCollection{
			Path: "/organizations", Prefix: "org", Object: "organization", Patch: true,
			OnCreate: func(o fakeObj, body map[string]any) error {
				if o.str("name") == "" {
					return errors.New("name is required")
				}
				if _, ok := o["slug"]; !ok {
					o["slug"] = strings.ToLower(strings.ReplaceAll(o.str("name"), " ", "-"))
				}
				if _, ok := o["max_allowed_memberships"]; !ok {
					o["max_allowed_memberships"] = 5
				}
				o["admin_delete_enabled"] = true
				for _, k := range []string{"public_metadata", "private_metadata"} {
					if _, ok := o[k]; !ok {
						o[k] = map[string]any{}
					}
				}
				return nil
			},
			OnUpdate: func(o fakeObj, body map[string]any) error {
				if _, ok := body["admin_delete_enabled"]; ok && o.str("name") == "failpatch" {
					return errors.New("cannot change admin_delete_enabled")
				}
				return nil
			},
		})
		// Clerk resolves an organization by ID or by slug in the same path position.
		f.mux.HandleFunc("GET /organizations/{idOrSlug}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			key := r.PathValue("idOrSlug")
			for _, o := range orgs.Objs {
				if o.str("id") == key || o.str("slug") == key {
					writeJSON(w, http.StatusOK, o)
					return
				}
			}
			notFound(w, "organization")
		})
	})
}

func TestOrganization_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("Organization", "o")

	inputs := pm(map[string]property.Value{
		"name":            str("Acme Corp"),
		"publicMetadata":  str(`{"b": 1, "a": {"y": 2, "x": 1}}`),
		"privateMetadata": str(`{"token":"t"}`).WithSecret(true),
	})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "acme-corp", getStr(t, created.Properties, "slug"))
	require.Equal(t, 5.0, created.Properties.Get("maxAllowedMemberships").AsNumber())
	require.True(t, created.Properties.Get("adminDeleteEnabled").AsBool())
	require.NotEmpty(t, getStr(t, created.Properties, "createdAt"))
	require.True(t, created.Properties.Get("privateMetadata").Secret(), "privateMetadata must be secret")
	// The user's own metadata text is kept, not reformatted.
	require.Equal(t, `{"b": 1, "a": {"y": 2, "x": 1}}`, getStr(t, created.Properties, "publicMetadata"))

	diff := func(state, newIn, oldIn property.Map) pgp.DiffResponse {
		d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: state, Inputs: newIn, OldInputs: oldIn})
		require.NoError(t, err)
		return d
	}

	// Idempotent diff, even though Clerk filled slug, member cap and admin delete.
	requireNoChanges(t, diff(created.Properties, inputs, inputs))
	// Metadata compares as JSON: key order and spacing are not a change.
	reordered := inputs.Set("publicMetadata", str(`{"a":{"x":1,"y":2},"b":1}`))
	requireNoChanges(t, diff(created.Properties, reordered, inputs))
	// Real changes.
	d := diff(created.Properties, inputs.Set("name", str("Acme Inc")), inputs)
	require.Equal(t, pgp.Update, d.DetailedDiff["name"].Kind)
	d = diff(created.Properties, inputs.Set("publicMetadata", str(`{"b":2}`)), inputs)
	require.Equal(t, pgp.Update, d.DetailedDiff["publicMetadata"].Kind)
	d = diff(created.Properties, inputs.Set("createdBy", str("user_1")), inputs)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["createdBy"].Kind)

	// Dry-run update predicts without calling the API.
	patches := f.callCount("PATCH /organizations")
	prev, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs, Inputs: inputs.Set("name", str("Acme Inc")), DryRun: true})
	require.NoError(t, err)
	require.Equal(t, "Acme Inc", getStr(t, prev.Properties, "name"))
	require.Equal(t, patches, f.callCount("PATCH /organizations"))

	// Update in place.
	newIn := inputs.Set("name", str("Acme Inc")).Set("adminDeleteEnabled", boolean(false)).Set("maxAllowedMemberships", num(25))
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs, Inputs: newIn})
	require.NoError(t, err)
	require.Equal(t, "Acme Inc", getStr(t, upd.Properties, "name"))
	require.False(t, upd.Properties.Get("adminDeleteEnabled").AsBool())
	require.Equal(t, 25.0, upd.Properties.Get("maxAllowedMemberships").AsNumber())
	requireNoChanges(t, diff(upd.Properties, newIn, newIn))

	// Refresh by ID keeps unset inputs unset and set ones in step with Clerk.
	f.collections["/organizations"].Objs[created.ID]["name"] = "Renamed Elsewhere"
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Equal(t, "Renamed Elsewhere", getStr(t, read.Properties, "name"))
	require.Equal(t, "Renamed Elsewhere", getStr(t, read.Inputs, "name"))
	_, hasSlug := read.Inputs.GetOk("slug")
	require.False(t, hasSlug, "slug was not set by the user and must stay out of inputs")
	require.Equal(t, `{"b": 1, "a": {"y": 2, "x": 1}}`, getStr(t, read.Properties, "publicMetadata"))

	// Import by ID alone, then no diff.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, created.ID, imp.ID)
	require.Equal(t, "acme-corp", getStr(t, imp.Inputs, "slug"))
	require.JSONEq(t, `{"b":1,"a":{"y":2,"x":1}}`, getStr(t, imp.Inputs, "publicMetadata"))
	requireNoChanges(t, diff(imp.Properties, imp.Inputs, imp.Inputs))

	// Lookup function.
	out, err := invoke(t, s, "getOrganization", map[string]property.Value{"organizationId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "Renamed Elsewhere", getStr(t, out, "name"))
	out, err = invoke(t, s, "getOrganization", map[string]property.Value{"slug": str("acme-corp")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "organizationId"))
	_, err = invoke(t, s, "getOrganization", map[string]property.Value{"organizationId": str("org_missing")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getOrganization", map[string]property.Value{})
	require.ErrorContains(t, err, "exactly one")

	// Deleted outside Pulumi: Read reports it gone, Delete is quiet.
	f.collections["/organizations"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestOrganization_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("Organization", "o")

	_, err := s.Create(pgp.CreateRequest{Urn: urn, DryRun: true, Properties: pm(map[string]property.Value{"name": str("Dry")})})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST /organizations"), "dry run must not call the API")

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"name": str("Real")})})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/organizations"].Objs)
}

// Setting adminDeleteEnabled needs a follow-up PATCH; when it fails the
// organization already exists, so it is reported as an init failure with the
// ID rather than leaked.
func TestOrganization_PostCreateFailureKeepsID(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("Organization", "o")
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{
		"name": str("failpatch"), "adminDeleteEnabled": boolean(false),
	})})
	require.Error(t, err)
	require.NotEmpty(t, created.ID, "the created organization's ID must be returned with the error")
	require.Len(t, f.collections["/organizations"].Objs, 1)
}
