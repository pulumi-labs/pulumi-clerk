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
	"strings"
	"testing"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		f.collection(&fakeCollection{
			Path: "/organization_permissions", Prefix: "perm", Object: "permission", ByID: true, Patch: true,
			OnCreate: func(o fakeObj, body map[string]any) error {
				if !strings.HasPrefix(o.str("key"), "org:") {
					return errors.New("key must start with org:")
				}
				o["type"] = "user"
				if _, ok := o["description"]; !ok {
					o["description"] = nil
				}
				return nil
			},
			OnUpdate: func(o fakeObj, body map[string]any) error {
				if !strings.HasPrefix(o.str("key"), "org:") {
					return errors.New("key must start with org:")
				}
				return nil
			},
		})
	})
}

func TestOrganizationPermission_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("OrganizationPermission", "p")

	inputs := pm(map[string]property.Value{"name": str("Create posts"), "key": str("org:posts:create"), "description": str("May create posts")})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "user", getStr(t, created.Properties, "type"))
	require.NotEmpty(t, getStr(t, created.Properties, "createdAt"))

	diff := func(state, newIn, oldIn property.Map) pgp.DiffResponse {
		d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: state, Inputs: newIn, OldInputs: oldIn})
		require.NoError(t, err)
		return d
	}
	requireNoChanges(t, diff(created.Properties, inputs, inputs))
	d := diff(created.Properties, inputs.Set("name", str("Create blog posts")), inputs)
	require.True(t, d.HasChanges)

	// Update in place, including dropping the description.
	newIn := pm(map[string]property.Value{"name": str("Create blog posts"), "key": str("org:posts:publish")})
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: newIn, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "Create blog posts", getStr(t, upd.Properties, "name"))
	require.Equal(t, "org:posts:publish", getStr(t, upd.Properties, "key"))
	_, hasDesc := upd.Properties.GetOk("description")
	require.False(t, hasDesc, "description cleared")
	requireNoChanges(t, diff(upd.Properties, newIn, newIn))

	// Refresh and import by ID alone.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Equal(t, "org:posts:publish", getStr(t, read.Properties, "key"))
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, created.ID, imp.ID)
	require.Equal(t, "Create blog posts", getStr(t, imp.Inputs, "name"))
	requireNoChanges(t, diff(imp.Properties, imp.Inputs, imp.Inputs))

	// Lookup function by ID and by key.
	out, err := invoke(t, s, "getOrganizationPermission", map[string]property.Value{"organizationPermissionId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "org:posts:publish", getStr(t, out, "key"))
	out, err = invoke(t, s, "getOrganizationPermission", map[string]property.Value{"key": str("org:posts:publish")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "organizationPermissionId"))
	_, err = invoke(t, s, "getOrganizationPermission", map[string]property.Value{"key": str("org:none")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getOrganizationPermission", map[string]property.Value{"organizationPermissionId": str("perm_missing")})
	require.ErrorContains(t, err, "was not found")

	// A rejected key is explained.
	_, err = s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: upd.Properties, Inputs: newIn.Set("key", str("bad")), OldInputs: newIn})
	require.ErrorContains(t, err, "org:")

	// Deleted outside Pulumi.
	f.collections["/organization_permissions"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestOrganizationPermission_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("OrganizationPermission", "p")
	in := pm(map[string]property.Value{"name": str("Read"), "key": str("org:docs:read")})

	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in, DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST /organization_permissions"), "dry run must not call the API")

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/organization_permissions"].Objs)
}
