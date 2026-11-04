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

// failDomainDelete makes the next domain deletes answer HTTP 500.
var failDomainDelete = "domain_delete_500"

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		domainsOf := func(app string) map[string]fakeObj {
			p := platformApps(f)
			if p.domains[app] == nil {
				p.domains[app] = map[string]fakeObj{}
			}
			return p.domains[app]
		}
		f.mux.HandleFunc("POST /platform/applications/{app}/domains", func(w http.ResponseWriter, r *http.Request) {
			body := readBody(r)
			f.mu.Lock()
			defer f.mu.Unlock()
			if _, ok := platformApps(f).apps[r.PathValue("app")]; !ok {
				notFound(w, "application")
				return
			}
			id := f.nextID("dmn")
			name, _ := body["name"].(string)
			d := fakeObj{"id": id, "name": name, "is_satellite": false, "is_provider_domain": false,
				"frontend_api_url": "https://clerk." + name,
				"cname_targets":    []any{map[string]any{"host": "clerk." + name, "value": "frontend-api.clerk.services", "required": true}}}
			domainsOf(r.PathValue("app"))[id] = d
			writeJSON(w, http.StatusOK, d)
		})
		f.mux.HandleFunc("GET /platform/applications/{app}/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			d, ok := domainsOf(r.PathValue("app"))[r.PathValue("id")]
			if !ok {
				notFound(w, "domain")
				return
			}
			writeJSON(w, http.StatusOK, d)
		})
		f.mux.HandleFunc("DELETE /platform/applications/{app}/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.state[failDomainDelete] != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal", "boom")
				return
			}
			m := domainsOf(r.PathValue("app"))
			if _, ok := m[r.PathValue("id")]; !ok {
				notFound(w, "domain")
				return
			}
			delete(m, r.PathValue("id"))
			writeJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "deleted": true})
		})
	})
}

func TestDomain_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	appURN := clerkUrn("Application", "app")
	app, err := s.Create(pgp.CreateRequest{Urn: appURN, Properties: pm(map[string]property.Value{"name": str("Acme")})})
	require.NoError(t, err)
	urn := clerkUrn("Domain", "d")
	inputs := pm(map[string]property.Value{"applicationId": str(app.ID), "name": str("app.example.com"), "proxyPath": str("/__clerk")})

	pre, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs, DryRun: true})
	require.NoError(t, err)
	require.Empty(t, pre.ID)
	require.Zero(t, f.callCount("POST /platform/applications/"+app.ID+"/domains"))

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.Contains(t, created.ID, app.ID+"/dmn_")
	require.Equal(t, "https://clerk.app.example.com", getStr(t, created.Properties, "frontendApiUrl"))
	targets := created.Properties.Get("cnameTargets").AsArray().AsSlice()
	require.Len(t, targets, 1)
	require.True(t, targets[0].AsMap().Get("required").AsBool())

	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Every input forces replacement.
	for k, v := range map[string]string{"name": "other.example.com", "applicationId": "app_other", "proxyPath": "/x"} {
		changed := pm(map[string]property.Value{"applicationId": str(app.ID), "name": str("app.example.com"), "proxyPath": str("/__clerk")})
		m := changed.AsMap()
		m[k] = str(v)
		d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs, Inputs: property.NewMap(m)})
		require.NoError(t, err)
		require.Equal(t, pgp.UpdateReplace, d.DetailedDiff[k].Kind, k)
	}

	// Refresh and import by composite ID alone.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "/__clerk", getStr(t, read.Inputs, "proxyPath"))
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, app.ID, getStr(t, imp.Inputs, "applicationId"))
	require.Equal(t, "app.example.com", getStr(t, imp.Inputs, "name"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)
	_, err = s.Read(pgp.ReadRequest{ID: "no-slash", Urn: urn})
	require.ErrorContains(t, err, "<applicationId>/<domainId>")

	// Lookup function.
	domainID := created.ID[len(app.ID)+1:]
	out, err := invoke(t, s, "getDomain", map[string]property.Value{"applicationId": str(app.ID), "domainId": str(domainID)})
	require.NoError(t, err)
	require.Equal(t, "app.example.com", getStr(t, out, "name"))
	_, err = invoke(t, s, "getDomain", map[string]property.Value{"applicationId": str(app.ID), "domainId": str("dmn_missing")})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi.
	f.mu.Lock()
	delete(platformApps(f).domains[app.ID], domainID)
	f.mu.Unlock()
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
}

func TestDomain_Delete(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	app, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("Application", "app"), Properties: pm(map[string]property.Value{"name": str("Acme")})})
	require.NoError(t, err)
	urn := clerkUrn("Domain", "d")
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"applicationId": str(app.ID), "name": str("a.example.com")})})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, platformApps(f).domains[app.ID])

	// The API's known HTTP 500 on domain delete is tolerated.
	again, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"applicationId": str(app.ID), "name": str("b.example.com")})})
	require.NoError(t, err)
	f.mu.Lock()
	f.state[failDomainDelete] = true
	f.mu.Unlock()
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: again.ID, Urn: urn, Properties: again.Properties}))
}
