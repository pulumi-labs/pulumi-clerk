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
	"net/http"
	"slices"
	"testing"
	"time"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// fakeRoleSets is an in-memory /role_sets API that enforces Clerk's rules:
// the default and creator roles must be in the set at all times.
type fakeRoleSets struct {
	f    *fakeClerk
	sets map[string]fakeObj
	// failName makes a PATCH that renames to this value fail.
	failName string
}

func strs(v any) []string {
	var out []string
	switch v := v.(type) {
	case []string:
		out = append(out, v...)
	case []any:
		for _, e := range v {
			out = append(out, e.(string))
		}
	}
	return out
}

// render builds the response shape, with roles sorted by key.
func (r *fakeRoleSets) render(o fakeObj) fakeObj {
	keys := strs(o["role_keys"])
	slices.Sort(keys)
	roles := []any{}
	for _, k := range keys {
		roles = append(roles, map[string]any{"object": "role", "key": k})
	}
	out := fakeObj{}
	for k, v := range o {
		if k != "role_keys" && k != "default_role_key" && k != "creator_role_key" {
			out[k] = v
		}
	}
	out["roles"] = roles
	out["default_role"] = map[string]any{"key": o["default_role_key"]}
	out["creator_role"] = map[string]any{"key": o["creator_role_key"]}
	return out
}

func (r *fakeRoleSets) lookup(idOrKey string) (fakeObj, bool) {
	for _, o := range r.sets {
		if o.str("id") == idOrKey || o.str("key") == idOrKey {
			return o, true
		}
	}
	return nil, false
}

func (r *fakeRoleSets) valid(o fakeObj) error {
	keys := strs(o["role_keys"])
	if len(keys) == 0 || len(keys) > 10 {
		return errors.New("a role set needs between 1 and 10 roles")
	}
	for _, k := range []string{"default_role_key", "creator_role_key"} {
		if !slices.Contains(keys, o.str(k)) {
			return errors.New(k + " must be one of the roles in the role set")
		}
	}
	return nil
}

func (r *fakeRoleSets) dropOutside(id string) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	delete(r.sets, id)
}

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		r := &fakeRoleSets{f: f, sets: map[string]fakeObj{}, failName: "boom"}
		f.state["rolesets"] = r
		f.mux.HandleFunc("POST /role_sets", func(w http.ResponseWriter, req *http.Request) {
			body := readBody(req)
			f.mu.Lock()
			defer f.mu.Unlock()
			now := time.Now().UnixMilli()
			typ, _ := body["type"].(string)
			if typ == "" {
				typ = "custom"
			}
			desc, ok := body["description"]
			if !ok {
				desc = nil
			}
			o := fakeObj{
				"object": "role_set", "id": f.nextID("rset"), "name": body["name"], "key": body["key"], "description": desc,
				"type": typ, "role_keys": body["roles"], "default_role_key": body["default_role_key"],
				"creator_role_key": body["creator_role_key"], "created_at": now, "updated_at": now,
			}
			if err := r.valid(o); err != nil {
				writeAPIError(w, http.StatusUnprocessableEntity, "form_param_invalid", err.Error())
				return
			}
			r.sets[o.str("id")] = o
			writeJSON(w, http.StatusOK, r.render(o))
		})
		f.mux.HandleFunc("GET /role_sets/{id}", func(w http.ResponseWriter, req *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			o, ok := r.lookup(req.PathValue("id"))
			if !ok {
				notFound(w, "role set")
				return
			}
			writeJSON(w, http.StatusOK, r.render(o))
		})
		f.mux.HandleFunc("PATCH /role_sets/{id}", func(w http.ResponseWriter, req *http.Request) {
			body := readBody(req)
			f.mu.Lock()
			defer f.mu.Unlock()
			o, ok := r.lookup(req.PathValue("id"))
			if !ok {
				notFound(w, "role set")
				return
			}
			if body["name"] == r.failName {
				writeAPIError(w, http.StatusUnprocessableEntity, "form_param_invalid", "name is not allowed")
				return
			}
			next := fakeObj{}
			for k, v := range o {
				next[k] = v
			}
			for _, k := range []string{"name", "key", "description", "type", "default_role_key", "creator_role_key"} {
				if v, ok := body[k]; ok {
					next[k] = v
				}
			}
			if err := r.valid(next); err != nil {
				writeAPIError(w, http.StatusUnprocessableEntity, "form_param_invalid", err.Error())
				return
			}
			next["updated_at"] = time.Now().UnixMilli()
			r.sets[o.str("id")] = next
			writeJSON(w, http.StatusOK, r.render(next))
		})
		f.mux.HandleFunc("POST /role_sets/{id}/roles", func(w http.ResponseWriter, req *http.Request) {
			body := readBody(req)
			f.mu.Lock()
			defer f.mu.Unlock()
			o, ok := r.lookup(req.PathValue("id"))
			if !ok {
				notFound(w, "role set")
				return
			}
			keys := strs(o["role_keys"])
			for _, k := range strs(body["role_keys"]) {
				if !slices.Contains(keys, k) {
					keys = append(keys, k)
				}
			}
			o["role_keys"] = keys
			writeJSON(w, http.StatusOK, r.render(o))
		})
		f.mux.HandleFunc("DELETE /role_sets/{id}/roles", func(w http.ResponseWriter, req *http.Request) {
			body := readBody(req)
			f.mu.Lock()
			defer f.mu.Unlock()
			o, ok := r.lookup(req.PathValue("id"))
			if !ok {
				notFound(w, "role set")
				return
			}
			drop := strs(body["role_keys"])
			next := fakeObj{}
			for k, v := range o {
				next[k] = v
			}
			var keep []string
			for _, k := range strs(o["role_keys"]) {
				if !slices.Contains(drop, k) {
					keep = append(keep, k)
				}
			}
			next["role_keys"] = keep
			if err := r.valid(next); err != nil {
				writeAPIError(w, http.StatusUnprocessableEntity, "form_param_invalid", err.Error())
				return
			}
			r.sets[o.str("id")] = next
			writeJSON(w, http.StatusOK, r.render(next))
		})
		f.mux.HandleFunc("DELETE /role_sets/{id}", func(w http.ResponseWriter, req *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			o, ok := r.lookup(req.PathValue("id"))
			if !ok {
				notFound(w, "role set")
				return
			}
			delete(r.sets, o.str("id"))
			writeJSON(w, http.StatusOK, map[string]any{"object": "deleted_object", "id": o.str("id"), "deleted": true})
		})
	})
}

func TestRoleSet_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	fr := f.state["rolesets"].(*fakeRoleSets)
	urn := clerkUrn("RoleSet", "rs")

	inputs := pm(map[string]property.Value{
		"name": str("Default"), "key": str("role_set:default"),
		"defaultRoleKey": str("org:member"), "creatorRoleKey": str("org:admin"),
		"roles": strArray("org:member", "org:admin"),
	})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	// Server default for type is reported, the user's role order is kept.
	require.Equal(t, "custom", getStr(t, created.Properties, "type"))
	require.Equal(t, []string{"org:member", "org:admin"}, strList(t, created.Properties, "roles"))
	require.Equal(t, "org:admin", getStr(t, created.Properties, "creatorRoleKey"))

	diff := func(state, newIn, oldIn property.Map) pgp.DiffResponse {
		d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: state, Inputs: newIn, OldInputs: oldIn})
		require.NoError(t, err)
		return d
	}
	// Idempotent despite the server-filled type; role order is not a change.
	requireNoChanges(t, diff(created.Properties, inputs, inputs))
	requireNoChanges(t, diff(created.Properties, inputs.Set("roles", strArray("org:admin", "org:member")), inputs))
	d := diff(created.Properties, inputs.Set("roles", strArray("org:member", "org:admin", "org:editor")), inputs)
	require.Equal(t, pgp.Update, d.DetailedDiff["roles"].Kind)
	d = diff(created.Properties, inputs.Set("defaultRoleKey", str("org:admin")), inputs)
	require.Equal(t, pgp.Update, d.DetailedDiff["defaultRoleKey"].Kind)

	// Swap the default role for a role not yet in the set and drop the old
	// one: roles must be added before the keys change and removed after.
	newIn := inputs.Set("defaultRoleKey", str("org:editor")).Set("roles", strArray("org:editor", "org:admin")).Set("name", str("Renamed"))
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: newIn, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "Renamed", getStr(t, upd.Properties, "name"))
	require.Equal(t, "org:editor", getStr(t, upd.Properties, "defaultRoleKey"))
	require.Equal(t, []string{"org:editor", "org:admin"}, strList(t, upd.Properties, "roles"))
	requireNoChanges(t, diff(upd.Properties, newIn, newIn))

	// A dry-run update does not touch the API.
	patches := f.callCount("PATCH /role_sets")
	_, err = s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: upd.Properties, Inputs: newIn.Set("name", str("Again")), OldInputs: newIn, DryRun: true})
	require.NoError(t, err)
	require.Equal(t, patches, f.callCount("PATCH /role_sets"))

	// Refresh and import by ID alone.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Equal(t, []string{"org:editor", "org:admin"}, strList(t, read.Properties, "roles"))
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, created.ID, imp.ID)
	require.Equal(t, "org:editor", getStr(t, imp.Inputs, "defaultRoleKey"))
	require.Equal(t, "org:admin", getStr(t, imp.Inputs, "creatorRoleKey"))
	require.ElementsMatch(t, []string{"org:editor", "org:admin"}, strList(t, imp.Inputs, "roles"))
	requireNoChanges(t, diff(imp.Properties, imp.Inputs, imp.Inputs))

	// Lookup function by ID and by key.
	out, err := invoke(t, s, "getRoleSet", map[string]property.Value{"roleSetId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "Renamed", getStr(t, out, "name"))
	require.Equal(t, "org:editor", getStr(t, out, "defaultRoleKey"))
	out, err = invoke(t, s, "getRoleSet", map[string]property.Value{"key": str("role_set:default")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "roleSetId"))
	_, err = invoke(t, s, "getRoleSet", map[string]property.Value{"key": str("role_set:none")})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi.
	fr.dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: newIn})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

// An update that fails after the role additions already landed is reported
// as a partial update rather than silently losing the applied step.
func TestRoleSet_PartialUpdateIsReported(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	fr := f.state["rolesets"].(*fakeRoleSets)
	urn := clerkUrn("RoleSet", "rs")
	inputs := pm(map[string]property.Value{
		"name": str("Default"), "key": str("role_set:default"), "type": str("custom"),
		"defaultRoleKey": str("org:member"), "creatorRoleKey": str("org:admin"),
		"roles": strArray("org:member", "org:admin"),
	})
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)

	newIn := inputs.Set("name", str("boom")).Set("roles", strArray("org:member", "org:admin", "org:editor"))
	_, err = s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: newIn, OldInputs: inputs})
	require.Error(t, err)
	require.ElementsMatch(t, []string{"org:member", "org:admin", "org:editor"}, strs(fr.sets[created.ID]["role_keys"]))
}

func TestRoleSet_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	fr := f.state["rolesets"].(*fakeRoleSets)
	urn := clerkUrn("RoleSet", "rs")
	in := pm(map[string]property.Value{
		"name": str("Small"), "key": str("role_set:small"), "defaultRoleKey": str("org:member"),
		"creatorRoleKey": str("org:member"), "roles": strArray("org:member"),
	})

	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in, DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST /role_sets"), "dry run must not call the API")

	// A default role outside the set is rejected with Clerk's explanation.
	_, err = s.Create(pgp.CreateRequest{Urn: urn, Properties: in.Set("defaultRoleKey", str("org:admin"))})
	require.ErrorContains(t, err, "default_role_key")

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: in})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, fr.sets)
}
