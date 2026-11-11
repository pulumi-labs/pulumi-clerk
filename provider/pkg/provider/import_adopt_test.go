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

func TestAdoptIfNotDefault(t *testing.T) {
	require.Nil(t, adoptIfNotDefault(60, 60))
	require.Equal(t, 120, *adoptIfNotDefault(120, 60))
	require.Nil(t, adoptIfNotDefault("RS256", "RS256"))
	require.Equal(t, "HS256", *adoptIfNotDefault("HS256", "RS256"))
	require.Nil(t, adoptIfNotDefault(true, true))
	require.False(t, *adoptIfNotDefault(false, true))
}

// An import of objects that hold Clerk's defaults adopts no optional input
// (#1563), and one that holds non-default values adopts them (#1801).
func TestImport_AdoptsOnlyNonDefaultOptionals(t *testing.T) {
	t.Run("machine", func(t *testing.T) {
		f := startFakeClerk(t)
		s := serverFor(t, f)
		urn := clerkUrn("Machine", "m")
		created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: machineInputs()})
		require.NoError(t, err)
		imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
		require.NoError(t, err)
		_, adopted := imp.Inputs.GetOk("defaultTokenTtl")
		require.False(t, adopted, "a default TTL must not be adopted (#1563)")

		withTTL, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"name": str("m2"), "defaultTokenTtl": num(7200)})})
		require.NoError(t, err)
		imp, err = s.Read(pgp.ReadRequest{ID: withTTL.ID, Urn: urn})
		require.NoError(t, err)
		require.Equal(t, float64(7200), imp.Inputs.Get("defaultTokenTtl").AsNumber())
	})
	t.Run("jwt template", func(t *testing.T) {
		f := startFakeClerk(t)
		s := serverFor(t, f)
		urn := clerkUrn("JwtTemplate", "j")
		created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: jwtInputs(`{"a":1}`)})
		require.NoError(t, err)
		imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
		require.NoError(t, err)
		for _, k := range []string{"lifetime", "allowedClockSkew", "customSigningKey", "signingAlgorithm"} {
			_, adopted := imp.Inputs.GetOk(k)
			require.False(t, adopted, "default %s must not be adopted (#1563)", k)
		}

		in := jwtInputs(`{"a":1}`)
		in = in.Set("lifetime", num(120))
		in = in.Set("allowedClockSkew", num(30))
		in = in.Set("signingAlgorithm", str("HS256"))
		custom, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in})
		require.NoError(t, err)
		imp, err = s.Read(pgp.ReadRequest{ID: custom.ID, Urn: urn})
		require.NoError(t, err)
		require.Equal(t, float64(120), imp.Inputs.Get("lifetime").AsNumber())
		require.Equal(t, float64(30), imp.Inputs.Get("allowedClockSkew").AsNumber())
		require.Equal(t, "HS256", getStr(t, imp.Inputs, "signingAlgorithm"))
	})
	t.Run("oauth application", func(t *testing.T) {
		f := startFakeClerk(t)
		s := serverFor(t, f)
		urn := clerkUrn("OauthApplication", "o")
		created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: oauthInputs()})
		require.NoError(t, err)
		imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
		require.NoError(t, err)
		_, adopted := imp.Inputs.GetOk("consentScreenEnabled")
		require.False(t, adopted, "a default consentScreenEnabled must not be adopted (#1563)")

		in := oauthInputs().Set("consentScreenEnabled", boolean(true))
		on, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in})
		require.NoError(t, err)
		imp, err = s.Read(pgp.ReadRequest{ID: on.ID, Urn: urn})
		require.NoError(t, err)
		require.Equal(t, true, imp.Inputs.Get("consentScreenEnabled").AsBool())
	})
	t.Run("application settings", func(t *testing.T) {
		f := startFakeClerk(t)
		s := serverFor(t, f)
		urn := clerkUrn("ApplicationSettings", "s")
		imp, err := s.Read(pgp.ReadRequest{ID: "application_settings", Urn: urn})
		require.NoError(t, err)
		for _, k := range []string{"maxAllowedMemberships", "adminDeleteEnabled", "domainsEnabled"} {
			_, adopted := imp.Inputs.GetOk(k)
			require.False(t, adopted, "default %s must not be adopted (#1563)", k)
		}

		orgSettings(f)["domains_enabled"] = true
		orgSettings(f)["admin_delete_enabled"] = false
		orgSettings(f)["max_allowed_memberships"] = float64(7)
		imp, err = s.Read(pgp.ReadRequest{ID: "application_settings", Urn: urn})
		require.NoError(t, err)
		require.Equal(t, true, imp.Inputs.Get("domainsEnabled").AsBool())
		require.Equal(t, false, imp.Inputs.Get("adminDeleteEnabled").AsBool())
		require.Equal(t, float64(7), imp.Inputs.Get("maxAllowedMemberships").AsNumber())
	})
}
