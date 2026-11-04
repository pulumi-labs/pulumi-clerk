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
		f.collection(&fakeCollection{Path: "/redirect_urls", Prefix: "ru", Object: "redirect_url", ByID: true})
	})
}

func TestRedirectUrl_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("RedirectUrl", "r")

	inputs := pm(map[string]property.Value{"url": str("https://app.example.com/callback")})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.NotEmpty(t, getStr(t, created.Properties, "createdAt"))

	// Idempotent diff.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Changing the URL replaces.
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs,
		Inputs: pm(map[string]property.Value{"url": str("https://other.example.com/callback")})})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["url"].Kind)

	// Refresh.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "https://app.example.com/callback", getStr(t, read.Properties, "url"))

	// Import by ID alone, then a no-change diff.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "https://app.example.com/callback", getStr(t, imp.Inputs, "url"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function by ID and by URL.
	out, err := invoke(t, s, "getRedirectUrl", map[string]property.Value{"redirectUrlId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "https://app.example.com/callback", getStr(t, out, "url"))
	out, err = invoke(t, s, "getRedirectUrl", map[string]property.Value{"url": str("https://app.example.com/callback")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "redirectUrlId"))
	_, err = invoke(t, s, "getRedirectUrl", map[string]property.Value{"url": str("https://nope.example.com")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getRedirectUrl", map[string]property.Value{"redirectUrlId": str("ru_missing")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getRedirectUrl", map[string]property.Value{})
	require.ErrorContains(t, err, "exactly one of")

	// Deleted outside Pulumi.
	f.collections["/redirect_urls"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
}

func TestRedirectUrl_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("RedirectUrl", "r")
	inputs := pm(map[string]property.Value{"url": str("https://app.example.com/cb")})

	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs, DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST"))

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/redirect_urls"].Objs)
}
