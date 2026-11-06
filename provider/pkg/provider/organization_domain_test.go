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
	"time"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// fakeDomains stores organization domains per organization.
type fakeDomains struct {
	f    *fakeClerk
	orgs map[string][]fakeObj
}

func (d *fakeDomains) dropDomain(orgID, id string) {
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	var keep []fakeObj
	for _, o := range d.orgs[orgID] {
		if o.str("id") != id {
			keep = append(keep, o)
		}
	}
	d.orgs[orgID] = keep
}

func (d *fakeDomains) dropOrg(orgID string) {
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	delete(d.orgs, orgID)
}

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		d := &fakeDomains{f: f, orgs: map[string][]fakeObj{}}
		f.state["domains"] = d
		status := func(verified bool) map[string]any {
			if verified {
				return map[string]any{"status": "verified", "strategy": "admin"}
			}
			return map[string]any{"status": "unverified", "strategy": "email_code"}
		}
		f.mux.HandleFunc("POST /organizations/{orgId}/domains", func(w http.ResponseWriter, r *http.Request) {
			body := readBody(r)
			f.mu.Lock()
			defer f.mu.Unlock()
			org := r.PathValue("orgId")
			mode, _ := body["enrollment_mode"].(string)
			if mode == "" {
				// Like the live API: the parameter is required (#1565).
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": []map[string]any{{
					"code": "form_param_value_invalid", "message": "is invalid",
					"long_message": "does not match the allowed values for parameter enrollment_mode. Allowed values: manual_invitation, automatic_invitation, automatic_suggestion or enterprise_sso",
				}}})
				return
			}
			verified, _ := body["verified"].(bool)
			now := time.Now().UnixMilli()
			o := fakeObj{
				"object": "organization_domain", "id": f.nextID("orgdmn"), "organization_id": org,
				"name": body["name"], "enrollment_mode": mode, "affiliation_email_address": nil,
				"verification": status(verified), "created_at": now, "updated_at": now,
			}
			d.orgs[org] = append(d.orgs[org], o)
			writeJSON(w, http.StatusOK, o)
		})
		f.mux.HandleFunc("GET /organizations/{orgId}/domains", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			items, ok := d.orgs[r.PathValue("orgId")]
			if !ok {
				notFound(w, "organization")
				return
			}
			if items == nil {
				items = []fakeObj{}
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": items, "total_count": len(items)})
		})
		f.mux.HandleFunc("PATCH /organizations/{orgId}/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
			body := readBody(r)
			f.mu.Lock()
			defer f.mu.Unlock()
			for _, o := range d.orgs[r.PathValue("orgId")] {
				if o.str("id") == r.PathValue("id") {
					if m, ok := body["enrollment_mode"]; ok {
						o["enrollment_mode"] = m
					}
					if v, ok := body["verified"].(bool); ok {
						o["verification"] = status(v)
					}
					o["updated_at"] = time.Now().UnixMilli()
					writeJSON(w, http.StatusOK, o)
					return
				}
			}
			notFound(w, "domain")
		})
		f.mux.HandleFunc("DELETE /organizations/{orgId}/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			org, id := r.PathValue("orgId"), r.PathValue("id")
			for i, o := range d.orgs[org] {
				if o.str("id") == id {
					d.orgs[org] = append(d.orgs[org][:i:i], d.orgs[org][i+1:]...)
					writeJSON(w, http.StatusOK, map[string]any{"object": "deleted_object", "id": id, "deleted": true})
					return
				}
			}
			notFound(w, "domain")
		})
	})
}

func TestOrganizationDomain_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	fd := f.state["domains"].(*fakeDomains)
	urn := clerkUrn("OrganizationDomain", "d")

	inputs := pm(map[string]property.Value{"organizationId": str("org_1"), "name": str("example.com")})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.Contains(t, created.ID, "org_1/orgdmn_")
	// Server-chosen values come back as outputs.
	require.Equal(t, "manual_invitation", getStr(t, created.Properties, "enrollmentMode"))
	require.Equal(t, "unverified", getStr(t, created.Properties, "verificationStatus"))
	require.False(t, created.Properties.Get("verified").AsBool())
	require.NotEmpty(t, getStr(t, created.Properties, "createdAt"))

	diff := func(state, newIn, oldIn property.Map) pgp.DiffResponse {
		d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: state, Inputs: newIn, OldInputs: oldIn})
		require.NoError(t, err)
		return d
	}
	// Idempotent despite the server-filled enrollment mode.
	requireNoChanges(t, diff(created.Properties, inputs, inputs))
	// organizationId and name replace; enrollmentMode updates.
	d := diff(created.Properties, inputs.Set("name", str("other.com")), inputs)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["name"].Kind)
	d = diff(created.Properties, inputs.Set("organizationId", str("org_2")), inputs)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["organizationId"].Kind)
	withMode := inputs.Set("enrollmentMode", str("automatic_invitation"))
	d = diff(created.Properties, withMode, inputs)
	require.Equal(t, pgp.Update, d.DetailedDiff["enrollmentMode"].Kind)

	// Update enrollment mode and verify the domain.
	newIn := withMode.Set("verified", boolean(true))
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: newIn, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "automatic_invitation", getStr(t, upd.Properties, "enrollmentMode"))
	require.Equal(t, "verified", getStr(t, upd.Properties, "verificationStatus"))
	require.True(t, upd.Properties.Get("verified").AsBool())
	requireNoChanges(t, diff(upd.Properties, newIn, newIn))

	// Refresh and import by composite ID alone.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Equal(t, "example.com", getStr(t, read.Properties, "name"))
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, created.ID, imp.ID)
	require.Equal(t, "org_1", getStr(t, imp.Inputs, "organizationId"))
	require.Equal(t, "example.com", getStr(t, imp.Inputs, "name"))
	requireNoChanges(t, diff(imp.Properties, imp.Inputs, imp.Inputs))
	// A malformed import ID is explained.
	_, err = s.Read(pgp.ReadRequest{ID: "orgdmn_only", Urn: urn})
	require.ErrorContains(t, err, "organizationId")

	// Lookup function by name and by ID, and the not-found error.
	out, err := invoke(t, s, "getOrganizationDomain", map[string]property.Value{"organizationId": str("org_1"), "name": str("example.com")})
	require.NoError(t, err)
	require.Equal(t, "automatic_invitation", getStr(t, out, "enrollmentMode"))
	require.True(t, out.Get("verified").AsBool())
	_, err = invoke(t, s, "getOrganizationDomain", map[string]property.Value{"organizationId": str("org_1"), "name": str("nope.com")})
	require.ErrorContains(t, err, "was not found")
	// #1566: the resource's own composite id and the bare id both match.
	_, bareID, _ := splitID("d", "o", created.ID)
	for _, id := range []string{created.ID, bareID} {
		out, err = invoke(t, s, "getOrganizationDomain", map[string]property.Value{"organizationId": str("org_1"), "organizationDomainId": str(id)})
		require.NoError(t, err, id)
		require.Equal(t, "example.com", getStr(t, out, "name"))
	}
	// An id that is only the organization prefix names no domain.
	_, err = invoke(t, s, "getOrganizationDomain", map[string]property.Value{"organizationId": str("org_1"), "organizationDomainId": str("org_1/")})
	require.ErrorContains(t, err, "names no domain")
	// An id of another organization does not match.
	_, err = invoke(t, s, "getOrganizationDomain", map[string]property.Value{"organizationId": str("org_1"), "organizationDomainId": str("org_2/" + bareID)})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi (domain, then whole organization): Read reports gone, Delete is quiet.
	_, domID, _ := splitID("d", "o", created.ID)
	fd.dropDomain("org_1", domID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
	fd.dropOrg("org_1")
	gone, err = s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestOrganizationDomain_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("OrganizationDomain", "d")
	in := pm(map[string]property.Value{"organizationId": str("org_1"), "name": str("example.com"), "verified": boolean(true)})

	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in, DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST /organizations/org_1/domains"), "dry run must not call the API")

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in})
	require.NoError(t, err)
	require.Equal(t, "verified", getStr(t, created.Properties, "verificationStatus"))
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.state["domains"].(*fakeDomains).orgs["org_1"])
}
