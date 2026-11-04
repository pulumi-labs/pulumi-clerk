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
	"testing"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		f.collection(&fakeCollection{
			Path: "/saml_connections", Prefix: "samlc", Object: "saml_connection", ByID: true, Patch: true,
			OnCreate: func(o fakeObj, body map[string]any) error {
				id := o.str("id")
				o["acs_url"] = "https://clerk.example.com/v1/saml/acs/" + id
				o["sp_entity_id"] = "https://clerk.example.com/v1/saml/" + id
				o["sp_metadata_url"] = "https://clerk.example.com/v1/saml/metadata/" + id
				o["user_count"] = 0
				// The create endpoint does not accept these; Clerk defaults them.
				for _, k := range []string{"active", "sync_user_attributes", "allow_subdomains", "allow_idp_initiated", "disable_additional_identifications"} {
					o[k] = false
				}
				o["sync_user_attributes"] = true
				if _, ok := o["force_authn"]; !ok {
					o["force_authn"] = false
				}
				if _, ok := o["attribute_mapping"]; !ok {
					o["attribute_mapping"] = map[string]any{"user_id": "userId", "email_address": "mail", "first_name": "firstName", "last_name": "lastName"}
				}
				return nil
			},
		})
	})
}

var errSamlBoom = errors.New("boom")

func samlInputs() property.Map {
	return pm(map[string]property.Value{
		"name":         str("Okta"),
		"domain":       str("example.com"),
		"providerType": str("saml_okta"),
		"idpEntityId":  str("http://www.okta.com/abc"),
		"idpSsoUrl":    str("https://example.okta.com/sso"),
		"active":       boolean(true),
		"attributeMapping": property.New(pm(map[string]property.Value{
			"userId": str("uid"), "emailAddress": str("email"), "firstName": str("fn"), "lastName": str("ln"),
		})),
	})
}

func TestSamlConnection_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("SamlConnection", "s")
	col := f.collections["/saml_connections"]

	inputs := samlInputs()
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "saml_okta", getStr(t, created.Properties, "providerType"))
	require.Contains(t, getStr(t, created.Properties, "acsUrl"), created.ID)
	require.True(t, created.Properties.Get("active").AsBool(), "the follow-up update applied active")
	require.Equal(t, 1, f.callCount("PATCH /saml_connections/"+created.ID))
	require.Equal(t, "uid", getStr(t, created.Properties.Get("attributeMapping").AsMap(), "userId"))

	// Idempotent diff.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// The provider type cannot change: replace.
	changed := inputs.Set("providerType", str("saml_custom"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: changed, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["providerType"].Kind)

	// Everything else updates in place.
	renamed := inputs.Set("name", str("Okta SSO")).Set("forceAuthn", boolean(true))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: renamed, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, pgp.Update, d.DetailedDiff["name"].Kind)
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: renamed, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "Okta SSO", getStr(t, upd.Properties, "name"))
	require.True(t, upd.Properties.Get("forceAuthn").AsBool())
	require.Equal(t, "Okta SSO", col.Objs[created.ID].str("name"))

	// Update previews do not call the API.
	before := f.callCount("PATCH ")
	_, err = s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: upd.Properties, Inputs: inputs, OldInputs: renamed, DryRun: true})
	require.NoError(t, err)
	require.Equal(t, before, f.callCount("PATCH "))

	// Refresh by ID keeps the user's unset optional inputs unset.
	minimal := pm(map[string]property.Value{"name": str("Plain"), "domain": str("plain.example.com"), "providerType": str("saml_custom")})
	m, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: minimal})
	require.NoError(t, err)
	require.Equal(t, 0, f.callCount("PATCH /saml_connections/"+m.ID), "no follow-up call without settings")
	read, err := s.Read(pgp.ReadRequest{ID: m.ID, Urn: urn, Properties: m.Properties, Inputs: minimal})
	require.NoError(t, err)
	for _, k := range []string{"active", "syncUserAttributes", "attributeMapping", "forceAuthn", "idpEntityId"} {
		_, ok := read.Inputs.GetOk(k)
		require.False(t, ok, "input %s must stay unset after refresh", k)
	}
	require.True(t, read.Properties.Get("effective").AsMap().Get("syncUserAttributes").AsBool(), "effective shows the server default")
	require.Equal(t, "mail", getStr(t, read.Properties.Get("effective").AsMap().Get("attributeMapping").AsMap(), "emailAddress"))
	d, err = s.Diff(pgp.DiffRequest{ID: m.ID, Urn: urn, State: read.Properties, Inputs: minimal, OldInputs: read.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Import by ID alone adopts what Clerk reports, then diffs clean.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, created.ID, imp.ID)
	require.Equal(t, "Okta SSO", getStr(t, imp.Inputs, "name"))
	require.Equal(t, "saml_okta", getStr(t, imp.Inputs, "providerType"))
	require.Equal(t, "uid", getStr(t, imp.Inputs.Get("attributeMapping").AsMap(), "userId"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function by ID and by domain.
	out, err := invoke(t, s, "getSamlConnection", map[string]property.Value{"samlConnectionId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "Okta SSO", getStr(t, out, "name"))
	out, err = invoke(t, s, "getSamlConnection", map[string]property.Value{"domain": str("plain.example.com")})
	require.NoError(t, err)
	require.Equal(t, m.ID, getStr(t, out, "samlConnectionId"))
	_, err = invoke(t, s, "getSamlConnection", map[string]property.Value{"domain": str("nope.example.com")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getSamlConnection", map[string]property.Value{"samlConnectionId": str("samlc_missing")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getSamlConnection", map[string]property.Value{})
	require.ErrorContains(t, err, "exactly one")

	// Deleted outside Pulumi: Read reports it gone, Delete is quiet.
	col.dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: renamed})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))

	// Plain delete.
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: m.ID, Urn: urn, Properties: m.Properties}))
	require.Empty(t, col.Objs)
}

func TestSamlConnection_DryRunDoesNotCallAPI(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	resp, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("SamlConnection", "s"), Properties: samlInputs(), DryRun: true})
	require.NoError(t, err)
	require.Empty(t, resp.ID)
	require.Equal(t, 0, f.callCount(""))
}

// A failure after the connection exists keeps it in state instead of leaking it.
func TestSamlConnection_PartialCreateIsKept(t *testing.T) {
	f := startFakeClerk(t)
	col := f.collections["/saml_connections"]
	col.OnUpdate = func(fakeObj, map[string]any) error { return errSamlBoom }
	s := serverFor(t, f)
	resp, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("SamlConnection", "s"), Properties: samlInputs()})
	require.Error(t, err)
	require.NotNil(t, resp.PartialState)
	require.NotEmpty(t, resp.ID)
	require.Len(t, col.Objs, 1)
	require.Contains(t, resp.PartialState.Reasons[0], "could not be applied")
}
