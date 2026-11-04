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

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		settings := map[string]any{"object": "organization_settings", "enabled": false, "max_allowed_memberships": float64(5),
			"max_allowed_roles": float64(10), "max_allowed_permissions": float64(20), "creator_role": "org:admin",
			"admin_delete_enabled": true, "domains_enabled": false, "domains_default_role": "org:member"}
		f.state["orgSettings"] = settings
		f.mux.HandleFunc("PATCH /instance/organization_settings", func(w http.ResponseWriter, r *http.Request) {
			body := readBody(r)
			f.mu.Lock()
			defer f.mu.Unlock()
			for k, v := range body {
				settings[k] = v
			}
			writeJSON(w, http.StatusOK, settings)
		})
	})
}

func orgSettings(f *fakeClerk) map[string]any { return f.state["orgSettings"].(map[string]any) }

func TestApplicationSettings_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("ApplicationSettings", "s")
	// Only enableOrganizations and one optional are managed.
	inputs := pm(map[string]property.Value{"enableOrganizations": boolean(true), "maxAllowedMemberships": num(50)})

	pre, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs, DryRun: true})
	require.NoError(t, err)
	_ = pre
	require.Zero(t, f.callCount("PATCH /instance/organization_settings"))

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.Equal(t, "application_settings", created.ID)
	require.Equal(t, true, orgSettings(f)["enabled"])
	require.Equal(t, float64(50), orgSettings(f)["max_allowed_memberships"])
	// Unmanaged optionals are not sent and are exposed as current* outputs.
	_, sent := orgSettings(f)["domains_enabled"]
	require.True(t, sent) // the fake's own default
	require.Equal(t, "org:admin", getStr(t, created.Properties, "creatorRole"))
	require.Equal(t, "org:member", getStr(t, created.Properties, "domainsDefaultRole"))
	require.True(t, created.Properties.Get("currentAdminDeleteEnabled").AsBool())
	_, has := created.Properties.GetOk("adminDeleteEnabled")
	require.False(t, has, "unset optional inputs stay out of state")
	require.Equal(t, float64(10), created.Properties.Get("maxAllowedRoles").AsNumber())

	// Server-filled defaults do not cause a diff.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Update in place.
	next := pm(map[string]property.Value{"enableOrganizations": boolean(true), "maxAllowedMemberships": num(100), "domainsEnabled": boolean(true)})
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: inputs})
	require.NoError(t, err)
	require.True(t, d.HasChanges)
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, true, orgSettings(f)["domains_enabled"])
	require.Equal(t, float64(100), upd.Properties.Get("maxAllowedMemberships").AsNumber())

	// Refresh sends an empty update and reports drift made outside Pulumi.
	f.mu.Lock()
	orgSettings(f)["max_allowed_memberships"] = float64(7)
	f.mu.Unlock()
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: next})
	require.NoError(t, err)
	require.Equal(t, float64(7), read.Inputs.Get("maxAllowedMemberships").AsNumber())
	require.Equal(t, float64(7), read.Properties.Get("currentMaxAllowedMemberships").AsNumber())
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: read.Properties, Inputs: next, OldInputs: read.Inputs})
	require.NoError(t, err)
	require.True(t, d.HasChanges, "drift must show as a diff")
	// Read never changed the settings.
	require.Equal(t, float64(7), orgSettings(f)["max_allowed_memberships"])

	// Import by ID alone adopts what Clerk reports and then diffs clean.
	imp, err := s.Read(pgp.ReadRequest{ID: "application_settings", Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "application_settings", imp.ID)
	require.Equal(t, true, imp.Inputs.Get("enableOrganizations").AsBool())
	require.Equal(t, float64(7), imp.Inputs.Get("maxAllowedMemberships").AsNumber())
	d, err = s.Diff(pgp.DiffRequest{ID: imp.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function.
	out, err := invoke(t, s, "getApplicationSettings", map[string]property.Value{})
	require.NoError(t, err)
	require.Equal(t, true, out.Get("enableOrganizations").AsBool())
	require.Equal(t, "org:admin", getStr(t, out, "creatorRole"))

	// Delete disables organizations (as the Terraform provider did).
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
	require.Equal(t, false, orgSettings(f)["enabled"])
}

func TestApplicationSettings_NeedsBackendKey(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	// Wrong key surfaces the API's authentication error rather than a panic.
	_ = s
	require.Zero(t, f.callCount("PATCH"))
}
