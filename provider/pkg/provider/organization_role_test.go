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
	"sort"
	"strings"
	"testing"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// expandRolePermissions turns the permission IDs a client sends into the
// permission objects Clerk returns, sorted by ID so the server's order
// differs from a user's.
func expandRolePermissions(o fakeObj) {
	var ids []string
	switch v := o["permissions"].(type) {
	case []any:
		for _, e := range v {
			switch e := e.(type) {
			case string:
				ids = append(ids, e)
			case map[string]any:
				ids = append(ids, e["id"].(string))
			}
		}
	case nil:
	}
	sort.Strings(ids)
	perms := make([]any, 0, len(ids))
	for _, id := range ids {
		perms = append(perms, map[string]any{"object": "permission", "id": id, "key": "org:" + id})
	}
	o["permissions"] = perms
}

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		f.collection(&fakeCollection{
			Path: "/organization_roles", Prefix: "role", Object: "role", ByID: true, Patch: true,
			OnCreate: func(o fakeObj, body map[string]any) error {
				if !strings.HasPrefix(o.str("key"), "org:") {
					return errors.New("key must start with org:")
				}
				o["is_creator_eligible"] = false
				if _, ok := o["description"]; !ok {
					o["description"] = nil
				}
				expandRolePermissions(o)
				return nil
			},
			OnUpdate: func(o fakeObj, body map[string]any) error {
				expandRolePermissions(o)
				return nil
			},
		})
	})
}

func strList(t *testing.T, m property.Map, key string) []string {
	t.Helper()
	v, ok := m.GetOk(key)
	if !ok {
		return nil
	}
	var out []string
	for _, e := range v.AsArray().AsSlice() {
		out = append(out, e.AsString())
	}
	return out
}

func TestOrganizationRole_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("OrganizationRole", "r")

	inputs := pm(map[string]property.Value{
		"name": str("Editor"), "key": str("org:editor"), "description": str("Edits things"),
		"permissions": strArray("perm_b", "perm_a"),
	})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.False(t, created.Properties.Get("isCreatorEligible").AsBool())
	// Clerk returns permissions sorted; the user's order is kept in state.
	require.Equal(t, []string{"perm_b", "perm_a"}, strList(t, created.Properties, "permissions"))
	require.ElementsMatch(t, []string{"perm_a", "perm_b"}, func() []string {
		var ids []string
		for _, p := range f.collections["/organization_roles"].Objs[created.ID]["permissions"].([]any) {
			ids = append(ids, p.(map[string]any)["id"].(string))
		}
		return ids
	}())

	diff := func(state, newIn, oldIn property.Map) pgp.DiffResponse {
		d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: state, Inputs: newIn, OldInputs: oldIn})
		require.NoError(t, err)
		return d
	}
	requireNoChanges(t, diff(created.Properties, inputs, inputs))
	// Permission order is not a change; membership is.
	requireNoChanges(t, diff(created.Properties, inputs.Set("permissions", strArray("perm_a", "perm_b")), inputs))
	d := diff(created.Properties, inputs.Set("permissions", strArray("perm_a")), inputs)
	require.Equal(t, pgp.Update, d.DetailedDiff["permissions"].Kind)
	d = diff(created.Properties, inputs.Set("name", str("Senior editor")), inputs)
	require.Equal(t, pgp.Update, d.DetailedDiff["name"].Kind)

	// Update: new permission set, description dropped.
	newIn := pm(map[string]property.Value{"name": str("Editor"), "key": str("org:editor"), "permissions": strArray("perm_c", "perm_a")})
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: newIn, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, []string{"perm_c", "perm_a"}, strList(t, upd.Properties, "permissions"))
	_, hasDesc := upd.Properties.GetOk("description")
	require.False(t, hasDesc)
	requireNoChanges(t, diff(upd.Properties, newIn, newIn))

	// Removing all permissions clears them on the server.
	bare := pm(map[string]property.Value{"name": str("Editor"), "key": str("org:editor")})
	d = diff(upd.Properties, bare, newIn)
	require.Equal(t, pgp.Update, d.DetailedDiff["permissions"].Kind)
	cleared, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: upd.Properties, Inputs: bare, OldInputs: newIn})
	require.NoError(t, err)
	require.Empty(t, strList(t, cleared.Properties, "permissions"))
	requireNoChanges(t, diff(cleared.Properties, bare, bare))

	// Refresh and import by ID alone.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: cleared.Properties, Inputs: bare})
	require.NoError(t, err)
	require.Equal(t, "org:editor", getStr(t, read.Properties, "key"))
	f.collections["/organization_roles"].Objs[created.ID]["permissions"] = []any{map[string]any{"id": "perm_z", "key": "org:z"}}
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, created.ID, imp.ID)
	require.Equal(t, []string{"perm_z"}, strList(t, imp.Inputs, "permissions"))
	requireNoChanges(t, diff(imp.Properties, imp.Inputs, imp.Inputs))

	// Lookup function by ID and by key.
	out, err := invoke(t, s, "getOrganizationRole", map[string]property.Value{"organizationRoleId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "org:editor", getStr(t, out, "key"))
	out, err = invoke(t, s, "getOrganizationRole", map[string]property.Value{"key": str("org:editor")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "organizationRoleId"))
	_, err = invoke(t, s, "getOrganizationRole", map[string]property.Value{"key": str("org:nope")})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi.
	f.collections["/organization_roles"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: cleared.Properties, Inputs: bare})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: cleared.Properties}))
}

func TestOrganizationRole_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("OrganizationRole", "r")
	in := pm(map[string]property.Value{"name": str("Viewer"), "key": str("org:viewer")})

	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in, DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST /organization_roles"), "dry run must not call the API")

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in})
	require.NoError(t, err)
	_, hasPerms := created.Properties.GetOk("permissions")
	require.False(t, hasPerms, "no permissions requested, none reported")
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/organization_roles"].Objs)

	_, err = s.Create(pgp.CreateRequest{Urn: urn, Properties: in.Set("key", str("bad"))})
	require.ErrorContains(t, err, "org:")
}
