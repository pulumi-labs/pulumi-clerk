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
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// Regression tests for #2012: after an import, a program that declares an optional input equal to Clerk's
// default must plan no change, while any other value still updates and a removed non-default still reverts.

type defaultCase struct {
	field      string
	def        property.Value // Clerk's default
	other      property.Value // a different value
	nonDefault property.Value // a value an import adopts
}

func checkDefaultDiff(t *testing.T, typ string, base property.Map, c defaultCase) {
	t.Helper()
	urn := clerkUrn(typ, "r")
	// create makes the object on a fresh fake, so objects of the same name never collide.
	create := func(in property.Map) (integration.Server, pgp.CreateResponse) {
		s := serverFor(t, startFakeClerk(t))
		out, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in})
		require.NoError(t, err)
		return s, out
	}
	diff := func(s integration.Server, id string, state, in, old property.Map) pgp.DiffResponse {
		d, err := s.Diff(pgp.DiffRequest{ID: id, Urn: urn, State: state, Inputs: in, OldInputs: old})
		require.NoError(t, err)
		return d
	}

	// Import: nothing is adopted for a default, and a program that declares the default plans no change.
	s, created := create(base)
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	_, adopted := imp.Inputs.GetOk(c.field)
	require.False(t, adopted, "the default is not adopted on import (#1563)")
	declared := base.Set(c.field, c.def)
	requireNoChanges(t, diff(s, created.ID, imp.Properties, declared, imp.Inputs))
	requireNoChanges(t, diff(s, created.ID, imp.Properties, base, imp.Inputs))
	requireNoChanges(t, diff(s, created.ID, created.Properties, declared, base))

	// A value other than the default is an update of that input only.
	d := diff(s, created.ID, imp.Properties, base.Set(c.field, c.other), imp.Inputs)
	require.True(t, d.HasChanges)
	require.Len(t, d.DetailedDiff, 1)
	require.Contains(t, d.DetailedDiff, c.field)

	// A program that stated the default from the start and then drops it plans no change.
	s2, made := create(declared)
	requireNoChanges(t, diff(s2, made.ID, made.Properties, base, declared))

	// One that stated a real value and then drops it reverts to Clerk's default.
	custom := base.Set(c.field, c.nonDefault)
	s3, cm := create(custom)
	d = diff(s3, cm.ID, cm.Properties, base, custom)
	require.Contains(t, d.DetailedDiff, c.field)
	require.Equal(t, pgp.Delete, d.DetailedDiff[c.field].Kind)
	requireNoChanges(t, diff(s3, cm.ID, cm.Properties, custom, custom))
}

func TestMachineDiff_DeclaredDefaultAfterImport(t *testing.T) {
	checkDefaultDiff(t, "Machine", machineInputs(), defaultCase{"defaultTokenTtl", num(3600), num(900), num(7200)})
}

func TestJwtTemplateDiff_DeclaredDefaultAfterImport(t *testing.T) {
	for _, c := range []defaultCase{
		{"lifetime", num(60), num(30), num(120)},
		{"allowedClockSkew", num(5), num(10), num(30)},
		{"signingAlgorithm", str("RS256"), str("HS256"), str("RS512")},
	} {
		t.Run(c.field, func(t *testing.T) { checkDefaultDiff(t, "JwtTemplate", jwtInputs(`{"a":1}`), c) })
	}
}

func TestApplicationSettingsDiff_DeclaredDefaultAfterImport(t *testing.T) {
	base := pm(map[string]property.Value{"enableOrganizations": boolean(true)})
	for _, c := range []defaultCase{
		{"maxAllowedMemberships", num(5), num(10), num(50)},
		{"adminDeleteEnabled", boolean(true), boolean(false), boolean(false)},
		{"domainsEnabled", boolean(false), boolean(true), boolean(true)},
	} {
		t.Run(c.field, func(t *testing.T) { checkDefaultDiff(t, "ApplicationSettings", base, c) })
	}
}

// The other fields keep their kinds: names and flags update in place.
func TestDefaultDiff_OtherFieldsStillUpdate(t *testing.T) {
	s := serverFor(t, startFakeClerk(t))
	for typ, c := range map[string]struct {
		base property.Map
		key  string
		val  property.Value
	}{
		"Machine":             {machineInputs(), "name", str("renamed")},
		"JwtTemplate":         {jwtInputs(`{"a":1}`), "claims", str(`{"b":2}`)},
		"ApplicationSettings": {pm(map[string]property.Value{"enableOrganizations": boolean(true)}), "enableOrganizations", boolean(false)},
	} {
		s2 := serverFor(t, startFakeClerk(t))
		out, err := s2.Create(pgp.CreateRequest{Urn: clerkUrn(typ, "r"), Properties: c.base})
		require.NoError(t, err)
		d, err := s.Diff(pgp.DiffRequest{ID: out.ID, Urn: clerkUrn(typ, "r"), State: out.Properties, Inputs: c.base.Set(c.key, c.val), OldInputs: c.base})
		require.NoError(t, err)
		require.Equal(t, pgp.Update, d.DetailedDiff[c.key].Kind, typ)
	}
}
