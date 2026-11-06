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

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		f.collection(&fakeCollection{
			Path: "/allowlist_identifiers", Prefix: "alw", Object: "allowlist_identifier",
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

func TestAllowlistIdentifier_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("AllowlistIdentifier", "b")

	inputs := pm(map[string]property.Value{"identifier": str("ok@example.com")})
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
		Inputs: pm(map[string]property.Value{"identifier": str("better@example.com")})})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["identifier"].Kind)

	// Refresh by ID.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "ok@example.com", getStr(t, read.Properties, "identifier"))

	// Import by ID alone adopts what Clerk reports.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "ok@example.com", getStr(t, imp.Inputs, "identifier"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function.
	out, err := invoke(t, s, "getAllowlistIdentifier", map[string]property.Value{"identifier": str("ok@example.com")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "allowlistIdentifierId"))
	_, err = invoke(t, s, "getAllowlistIdentifier", map[string]property.Value{"identifier": str("nobody@example.com")})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi: Read reports the resource gone, Delete is quiet.
	f.collections["/allowlist_identifiers"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
}

func TestAllowlistIdentifier_Delete(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("AllowlistIdentifier", "b")
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"identifier": str("+15555550100")})})
	require.NoError(t, err)
	require.Equal(t, "phone_number", getStr(t, created.Properties, "identifierType"))
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/allowlist_identifiers"].Objs)
}

// notify is write-only: Clerk does not report it, so Read keeps what the
// program set, and changing it replaces the entry.
func TestAllowlistIdentifier_Notify(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("AllowlistIdentifier", "a")
	inputs := pm(map[string]property.Value{"identifier": str("*@example.com"), "notify": boolean(true)})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.Equal(t, true, f.collections["/allowlist_identifiers"].Objs[created.ID]["notify"])

	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	n, ok := read.Inputs.GetOk("notify")
	require.True(t, ok)
	require.True(t, n.AsBool())

	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: read.Properties, Inputs: inputs, OldInputs: read.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs,
		Inputs: pm(map[string]property.Value{"identifier": str("*@example.com"), "notify": boolean(false)})})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["notify"].Kind)
	require.True(t, d.DeleteBeforeReplace, "same identifier: Clerk rejects a duplicate, so delete first")
}

// #1562: Clerk never reports notify, so an imported entry has none recorded and
// a program that sets it must not replace the entry.
func TestAllowlistIdentifier_ImportThenSetNotifyIsNoChange(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("AllowlistIdentifier", "a")
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"identifier": str("a@example.com")})})
	require.NoError(t, err)
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	_, has := imp.Inputs.GetOk("notify")
	require.False(t, has)
	for _, notify := range []bool{false, true} {
		program := pm(map[string]property.Value{"identifier": str("a@example.com"), "notify": boolean(notify)})
		d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: program, OldInputs: imp.Inputs})
		require.NoError(t, err)
		requireNoChanges(t, d)
	}
	// A different identifier still replaces.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, OldInputs: imp.Inputs,
		Inputs: pm(map[string]property.Value{"identifier": str("b@example.com")})})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["identifier"].Kind)
}

// A dry run never calls the API.
func TestAllowlistIdentifier_DryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	_, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("AllowlistIdentifier", "a"), DryRun: true,
		Properties: pm(map[string]property.Value{"identifier": str("x@example.com")})})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST"))
	require.Empty(t, f.collections["/allowlist_identifiers"].Objs)
}
