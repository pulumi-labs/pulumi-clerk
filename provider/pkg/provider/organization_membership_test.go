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
	"strings"
	"testing"
	"time"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// fakeMemberships stores organization memberships per organization.
type fakeMemberships struct {
	f    *fakeClerk
	orgs map[string][]fakeObj
}

func (m *fakeMemberships) userID(o fakeObj) string {
	pud, _ := o["public_user_data"].(map[string]any)
	s, _ := pud["user_id"].(string)
	return s
}

func (m *fakeMemberships) dropUser(orgID, userID string) {
	m.f.mu.Lock()
	defer m.f.mu.Unlock()
	var keep []fakeObj
	for _, o := range m.orgs[orgID] {
		if m.userID(o) != userID {
			keep = append(keep, o)
		}
	}
	m.orgs[orgID] = keep
}

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		m := &fakeMemberships{f: f, orgs: map[string][]fakeObj{}}
		f.state["memberships"] = m
		roleNames := map[string]string{"org:admin": "Admin", "org:member": "Member", "org:editor": "Editor"}
		f.mux.HandleFunc("POST /organizations/{orgId}/memberships", func(w http.ResponseWriter, r *http.Request) {
			body := readBody(r)
			f.mu.Lock()
			defer f.mu.Unlock()
			org := r.PathValue("orgId")
			user, _ := body["user_id"].(string)
			role, _ := body["role"].(string)
			if _, ok := roleNames[role]; !ok {
				writeAPIError(w, http.StatusUnprocessableEntity, "form_param_invalid", "unknown role "+role)
				return
			}
			now := time.Now().UnixMilli()
			o := fakeObj{
				"object": "organization_membership", "id": f.nextID("orgmem"), "role": role, "role_name": roleNames[role],
				"organization": map[string]any{"id": org}, "public_user_data": map[string]any{"user_id": user},
				"created_at": now, "updated_at": now,
			}
			m.orgs[org] = append(m.orgs[org], o)
			writeJSON(w, http.StatusOK, o)
		})
		f.mux.HandleFunc("GET /organizations/{orgId}/memberships", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			items, ok := m.orgs[r.PathValue("orgId")]
			if !ok {
				notFound(w, "organization")
				return
			}
			out := []fakeObj{}
			for _, o := range items {
				if want := r.URL.Query()["user_id"]; len(want) > 0 && !strings.Contains(strings.Join(want, ","), m.userID(o)) {
					continue
				}
				out = append(out, o)
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": out, "total_count": len(out)})
		})
		f.mux.HandleFunc("PATCH /organizations/{orgId}/memberships/{userId}", func(w http.ResponseWriter, r *http.Request) {
			body := readBody(r)
			f.mu.Lock()
			defer f.mu.Unlock()
			for _, o := range m.orgs[r.PathValue("orgId")] {
				if m.userID(o) == r.PathValue("userId") {
					role, _ := body["role"].(string)
					if _, ok := roleNames[role]; !ok {
						writeAPIError(w, http.StatusUnprocessableEntity, "form_param_invalid", "unknown role "+role)
						return
					}
					o["role"], o["role_name"], o["updated_at"] = role, roleNames[role], time.Now().UnixMilli()
					writeJSON(w, http.StatusOK, o)
					return
				}
			}
			notFound(w, "membership")
		})
		f.mux.HandleFunc("DELETE /organizations/{orgId}/memberships/{userId}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			org := r.PathValue("orgId")
			for i, o := range m.orgs[org] {
				if m.userID(o) == r.PathValue("userId") {
					m.orgs[org] = append(m.orgs[org][:i:i], m.orgs[org][i+1:]...)
					writeJSON(w, http.StatusOK, o)
					return
				}
			}
			notFound(w, "membership")
		})
	})
}

func TestOrganizationMembership_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	fm := f.state["memberships"].(*fakeMemberships)
	urn := clerkUrn("OrganizationMembership", "m")

	inputs := pm(map[string]property.Value{"organizationId": str("org_1"), "userId": str("user_1"), "role": str("org:member")})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.Equal(t, "org_1/user_1", created.ID)
	require.Equal(t, "Member", getStr(t, created.Properties, "roleName"))
	require.Contains(t, getStr(t, created.Properties, "membershipId"), "orgmem_")

	// A second member must not be confused with the first on read.
	_, err = s.Create(pgp.CreateRequest{Urn: clerkUrn("OrganizationMembership", "m2"), Properties: inputs.Set("userId", str("user_2"))})
	require.NoError(t, err)

	diff := func(state, newIn, oldIn property.Map) pgp.DiffResponse {
		d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: state, Inputs: newIn, OldInputs: oldIn})
		require.NoError(t, err)
		return d
	}
	requireNoChanges(t, diff(created.Properties, inputs, inputs))
	d := diff(created.Properties, inputs.Set("userId", str("user_3")), inputs)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["userId"].Kind)
	d = diff(created.Properties, inputs.Set("organizationId", str("org_2")), inputs)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["organizationId"].Kind)
	d = diff(created.Properties, inputs.Set("role", str("org:admin")), inputs)
	require.Equal(t, pgp.Update, d.DetailedDiff["role"].Kind)

	// Update the role in place.
	newIn := inputs.Set("role", str("org:admin"))
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: newIn, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "org:admin", getStr(t, upd.Properties, "role"))
	require.Equal(t, "Admin", getStr(t, upd.Properties, "roleName"))
	requireNoChanges(t, diff(upd.Properties, newIn, newIn))
	// A rejected role leaves a clear error.
	_, err = s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: upd.Properties, Inputs: inputs.Set("role", str("org:nope")), OldInputs: newIn})
	require.ErrorContains(t, err, "unknown role")

	// Refresh and import by composite ID alone.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Equal(t, "org:admin", getStr(t, read.Properties, "role"))
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "user_1", getStr(t, imp.Inputs, "userId"))
	require.Equal(t, "org_1", getStr(t, imp.Inputs, "organizationId"))
	requireNoChanges(t, diff(imp.Properties, imp.Inputs, imp.Inputs))

	// Lookup function.
	out, err := invoke(t, s, "getOrganizationMembership", map[string]property.Value{"organizationId": str("org_1"), "userId": str("user_1")})
	require.NoError(t, err)
	require.Equal(t, "org:admin", getStr(t, out, "role"))
	require.Equal(t, "org_1/user_1", getStr(t, out, "organizationMembershipId"))
	_, err = invoke(t, s, "getOrganizationMembership", map[string]property.Value{"organizationId": str("org_1"), "userId": str("user_9")})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi.
	fm.dropUser("org_1", "user_1")
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestOrganizationMembership_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("OrganizationMembership", "m")
	in := pm(map[string]property.Value{"organizationId": str("org_1"), "userId": str("user_1"), "role": str("org:member")})

	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in, DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST /organizations/org_1/memberships"), "dry run must not call the API")

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.state["memberships"].(*fakeMemberships).orgs["org_1"])
}
