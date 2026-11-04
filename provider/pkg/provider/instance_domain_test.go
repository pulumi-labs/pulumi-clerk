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

// The instance Backend API has no get-by-ID for domains, only a list.
func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		f.collection(&fakeCollection{
			Path: "/domains", Prefix: "dmn", Object: "domain", Patch: true,
			OnCreate: func(o fakeObj, body map[string]any) error {
				name := o.str("name")
				if _, ok := o["is_satellite"]; !ok {
					o["is_satellite"] = false
				}
				o["frontend_api_url"] = "clerk." + name
				o["development_origin"] = ""
				if sat, _ := o["is_satellite"].(bool); !sat {
					o["accounts_portal_url"] = "accounts." + name
				}
				o["cname_targets"] = []map[string]any{{"host": "clerk." + name, "value": "frontend-api.clerk.services"}}
				return nil
			},
		})
	})
}

func TestInstanceDomain_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("InstanceDomain", "d")
	col := f.collections["/domains"]

	// Only the name is set: Clerk defaults isSatellite.
	inputs := pm(map[string]property.Value{"name": str("app.example.com")})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "clerk.app.example.com", getStr(t, created.Properties, "frontendApiUrl"))
	require.Equal(t, "accounts.app.example.com", getStr(t, created.Properties, "accountsPortalUrl"))
	_, set := created.Properties.GetOk("isSatellite")
	require.False(t, set, "the unset input stays unset")
	require.False(t, created.Properties.Get("effectiveIsSatellite").AsBool())
	require.Len(t, created.Properties.Get("cnameTargets").AsArray().AsSlice(), 1)

	// Idempotent diff.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Satellite is fixed at creation: replace. Name and proxy update in place.
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs,
		Inputs: inputs.Set("isSatellite", boolean(true))})
	require.NoError(t, err)
	require.Equal(t, pgp.AddReplace, d.DetailedDiff["isSatellite"].Kind)
	upIn := pm(map[string]property.Value{"name": str("app2.example.com"), "proxyUrl": str("https://app2.example.com/__clerk")})
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs, Inputs: upIn})
	require.NoError(t, err)
	require.Equal(t, pgp.Update, d.DetailedDiff["name"].Kind)
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: upIn, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "app2.example.com", getStr(t, upd.Properties, "name"))
	require.Equal(t, "https://app2.example.com/__clerk", getStr(t, upd.Properties, "proxyUrl"))

	// Refresh keeps the unset isSatellite input unset.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: upIn})
	require.NoError(t, err)
	_, ok := read.Inputs.GetOk("isSatellite")
	require.False(t, ok)
	require.Equal(t, "app2.example.com", getStr(t, read.Inputs, "name"))

	// Import by ID alone, then a clean diff.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "app2.example.com", getStr(t, imp.Inputs, "name"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function by ID and by name.
	out, err := invoke(t, s, "getInstanceDomain", map[string]property.Value{"instanceDomainId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "app2.example.com", getStr(t, out, "name"))
	out, err = invoke(t, s, "getInstanceDomain", map[string]property.Value{"name": str("app2.example.com")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "instanceDomainId"))
	_, err = invoke(t, s, "getInstanceDomain", map[string]property.Value{"name": str("nope.example.com")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getInstanceDomain", map[string]property.Value{})
	require.ErrorContains(t, err, "exactly one")

	// Deleted outside Pulumi.
	col.dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: upIn})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestInstanceDomain_SatelliteAndDelete(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("InstanceDomain", "d")
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{
		"name": str("sat.example.com"), "isSatellite": boolean(true)})})
	require.NoError(t, err)
	require.True(t, created.Properties.Get("isSatellite").AsBool())
	_, ok := created.Properties.GetOk("accountsPortalUrl")
	require.False(t, ok, "satellite domains have no accounts portal")
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/domains"].Objs)

	// Dry run does not call the API.
	n := f.callCount("")
	_, err = s.Create(pgp.CreateRequest{Urn: urn, DryRun: true, Properties: pm(map[string]property.Value{"name": str("x.example.com")})})
	require.NoError(t, err)
	require.Equal(t, n, f.callCount(""))
}
