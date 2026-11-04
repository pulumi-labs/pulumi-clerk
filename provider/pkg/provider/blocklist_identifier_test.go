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
	"strings"
	"testing"

	"github.com/blang/semver"
	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		f.collection(&fakeCollection{
			Path: "/blocklist_identifiers", Prefix: "blk", Object: "blocklist_identifier",
			OnCreate: func(o fakeObj, body map[string]any) error {
				ident := o.str("identifier")
				o["identifier_type"] = "email_address"
				if !strings.Contains(ident, "@") {
					o["identifier_type"] = "phone_number"
				}
				return nil
			},
		})
	})
}

func TestBlocklistIdentifier_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("BlocklistIdentifier", "b")

	inputs := pm(map[string]property.Value{"identifier": str("bad@example.com")})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "email_address", getStr(t, created.Properties, "identifierType"))
	require.NotEmpty(t, getStr(t, created.Properties, "createdAt"))

	// Idempotent diff: inputs equal to state report no changes.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Changing the identifier replaces (the API has no update).
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs,
		Inputs: pm(map[string]property.Value{"identifier": str("worse@example.com")})})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["identifier"].Kind)

	// Refresh by ID.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "bad@example.com", getStr(t, read.Properties, "identifier"))

	// Import by ID alone adopts what Clerk reports.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "bad@example.com", getStr(t, imp.Inputs, "identifier"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function.
	out, err := invoke(t, s, "getBlocklistIdentifier", map[string]property.Value{"identifier": str("bad@example.com")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "blocklistIdentifierId"))
	_, err = invoke(t, s, "getBlocklistIdentifier", map[string]property.Value{"identifier": str("nobody@example.com")})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi: Read reports the resource gone, Delete is quiet.
	f.collections["/blocklist_identifiers"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
}

func TestBlocklistIdentifier_Delete(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("BlocklistIdentifier", "b")
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"identifier": str("+15555550100")})})
	require.NoError(t, err)
	require.Equal(t, "phone_number", getStr(t, created.Properties, "identifierType"))
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/blocklist_identifiers"].Objs)
}

// A provider configured without keys explains how to set them.
func TestMissingKeysAreExplained(t *testing.T) {
	for _, v := range []string{"CLERK_API_KEY", "CLERK_SECRET_KEY", "CLERK_PLATFORM_API_KEY", "CLERK_API_URL"} {
		t.Setenv(v, "")
	}
	p, err := New()
	require.NoError(t, err)
	s, err := integration.NewServer(t.Context(), "clerk", semver.MustParse("0.0.1"), integration.WithProvider(p))
	require.NoError(t, err)
	require.NoError(t, s.Configure(pgp.ConfigureRequest{Args: pm(map[string]property.Value{})}))
	_, err = s.Create(pgp.CreateRequest{Urn: clerkUrn("BlocklistIdentifier", "b"),
		Properties: pm(map[string]property.Value{"identifier": str("x@example.com")})})
	require.ErrorContains(t, err, "clerk:apiKey")
}
