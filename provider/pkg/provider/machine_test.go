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
	"encoding/json"
	"net/http"
	"testing"

	"github.com/pulumi/pulumi-go-provider/integration"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

const (
	machineSecret    = "ak_machine_secret_value"
	machineSecretKey = "machine.secretFailure"
)

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		// Machines return their secret key on create only (as "secret_key")
		// and from a dedicated endpoint at any time.
		c := f.collection(&fakeCollection{
			Path: "/machines", Prefix: "mch", Object: "machine", ByID: true, Patch: true,
			OnCreate: func(o fakeObj, body map[string]any) error {
				if _, ok := o["default_token_ttl"]; !ok {
					o["default_token_ttl"] = float64(3600)
				}
				if f.state["machine.inline"] == true {
					o["secret_key"] = "ak_inline_secret"
				}
				return nil
			},
		})
		_ = c
		f.mux.HandleFunc("GET /machines/{id}/secret_key", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if _, ok := f.collections["/machines"].Objs[r.PathValue("id")]; !ok {
				notFound(w, "machine")
				return
			}
			if f.state[machineSecretKey] == true {
				writeAPIError(w, http.StatusForbidden, "forbidden", "not allowed to read secrets")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"object": "machine_secret_key", "secret": machineSecret})
		})
	})
}

// By default the fake omits secret_key from the create response, so Create
// falls back to the secret endpoint; TestMachine_InlineSecret covers the
// real-world shape where it comes back inline.

// requireSecretInSchema asserts that the named properties of a resource are
// declared secret in the provider schema, which is what makes the engine treat
// the values as secrets in state and logs.
func requireSecretInSchema(t *testing.T, s integration.Server, resource string, props ...string) {
	t.Helper()
	resp, err := s.GetSchema(pgp.GetSchemaRequest{})
	require.NoError(t, err)
	var spec struct {
		Resources map[string]struct {
			Properties map[string]struct {
				Secret bool `json:"secret"`
			} `json:"properties"`
			InputProperties map[string]struct {
				Secret bool `json:"secret"`
			} `json:"inputProperties"`
		} `json:"resources"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp.Schema), &spec))
	r, ok := spec.Resources["clerk:index:"+resource]
	require.True(t, ok, "resource %s missing from schema", resource)
	for _, p := range props {
		require.True(t, r.Properties[p].Secret, "output %s.%s must be secret in the schema", resource, p)
	}
}

func machineInputs() property.Map {
	return pm(map[string]property.Value{"name": str("ci-bot")})
}

func TestMachine_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("Machine", "m")

	inputs := machineInputs()
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, float64(3600), created.Properties.Get("effectiveDefaultTokenTtl").AsNumber())
	_, hasTTL := created.Properties.GetOk("defaultTokenTtl")
	require.False(t, hasTTL)
	require.Equal(t, machineSecret, created.Properties.Get("secretKey").AsString())
	requireSecretInSchema(t, s, "Machine", "secretKey")

	// Idempotent diff.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Refresh keeps the secret without calling the secret endpoint again.
	before := f.callCount("GET /machines/" + created.ID + "/secret_key")
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Equal(t, machineSecret, read.Properties.Get("secretKey").AsString())
	require.Equal(t, before, f.callCount("GET /machines/"+created.ID+"/secret_key"))
	_, hasTTL = read.Inputs.GetOk("defaultTokenTtl")
	require.False(t, hasTTL)

	// Update in place; the secret survives.
	next := pm(map[string]property.Value{"name": str("ci-bot-2"), "defaultTokenTtl": num(900)})
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: inputs})
	require.NoError(t, err)
	require.True(t, d.HasChanges)
	require.NotEqual(t, pgp.UpdateReplace, d.DetailedDiff["name"].Kind)
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: next, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "ci-bot-2", getStr(t, upd.Properties, "name"))
	require.Equal(t, float64(900), upd.Properties.Get("effectiveDefaultTokenTtl").AsNumber())
	require.Equal(t, machineSecret, upd.Properties.Get("secretKey").AsString())

	// Import by ID alone recovers the secret and diffs cleanly.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "ci-bot-2", getStr(t, imp.Inputs, "name"))
	require.Equal(t, machineSecret, imp.Properties.Get("secretKey").AsString())
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function never exposes the secret.
	out, err := invoke(t, s, "getMachine", map[string]property.Value{"name": str("ci-bot-2")})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "machineId"))
	_, leaked := out.GetOk("secretKey")
	require.False(t, leaked)
	out, err = invoke(t, s, "getMachine", map[string]property.Value{"machineId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, "ci-bot-2", getStr(t, out, "name"))
	_, err = invoke(t, s, "getMachine", map[string]property.Value{"name": str("nobody")})
	require.ErrorContains(t, err, "was not found")
	_, err = invoke(t, s, "getMachine", map[string]property.Value{"machineId": str("mch_missing")})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi.
	f.collections["/machines"].dropOutside(created.ID)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: next})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestMachine_DeleteAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("Machine", "m")
	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: machineInputs(), DryRun: true})
	require.NoError(t, err)
	require.Zero(t, f.callCount("POST"))
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: machineInputs()})
	require.NoError(t, err)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, f.collections["/machines"].Objs)
}

// When the machine exists but its secret key cannot be read, Create reports a
// partial failure with the ID so the machine is tracked rather than leaked.
func TestMachine_PartialCreate(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	f.state[machineSecretKey] = true
	_, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("Machine", "m"), Properties: machineInputs()})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to initialize")
	require.Len(t, f.collections["/machines"].Objs, 1)
}

func TestMachine_InlineSecret(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	f.state["machine.inline"] = true
	created, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("Machine", "m"), Properties: machineInputs()})
	require.NoError(t, err)
	require.Equal(t, "ak_inline_secret", created.Properties.Get("secretKey").AsString())
	require.Zero(t, f.callCount("GET /machines/"+created.ID+"/secret_key"))
}
