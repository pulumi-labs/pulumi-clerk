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
	"fmt"
	"net/http"
	"testing"

	"github.com/blang/semver"
	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// fakeInstances holds instance configurations by "<app>/<instance>".
type fakeInstances struct {
	configs map[string]map[string]any
	version int
}

func fakeInstancesOf(f *fakeClerk) *fakeInstances { return f.state["instances"].(*fakeInstances) }

// seedInstance creates an instance with a configuration as if it pre-existed.
func (fi *fakeInstances) seed(f *fakeClerk, app, inst string, cfg map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fi.configs[app+"/"+inst] = cfg
}

// mergeConfig applies a destructive PATCH: null removes a key, objects merge.
func mergeConfig(dst, patch map[string]any) {
	for k, v := range patch {
		switch pv := v.(type) {
		case nil:
			delete(dst, k)
		case map[string]any:
			if dm, ok := dst[k].(map[string]any); ok {
				mergeConfig(dm, pv)
			} else {
				dst[k] = pv
			}
		default:
			dst[k] = v
		}
	}
}

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		fi := &fakeInstances{configs: map[string]map[string]any{}}
		f.state["instances"] = fi
		reply := func(w http.ResponseWriter, cfg map[string]any) {
			out := map[string]any{"config_version": fmt.Sprintf("v%d", fi.version)}
			for k, v := range cfg {
				out[k] = v
			}
			writeJSON(w, http.StatusOK, out)
		}
		f.mux.HandleFunc("GET /platform/applications/{app}/instances/{inst}/config", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			cfg, ok := fi.configs[r.PathValue("app")+"/"+r.PathValue("inst")]
			if !ok {
				notFound(w, "instance")
				return
			}
			reply(w, cfg)
		})
		f.mux.HandleFunc("PATCH /platform/applications/{app}/instances/{inst}/config", func(w http.ResponseWriter, r *http.Request) {
			patch := readBody(r)
			f.mu.Lock()
			defer f.mu.Unlock()
			cfg, ok := fi.configs[r.PathValue("app")+"/"+r.PathValue("inst")]
			if !ok {
				notFound(w, "instance")
				return
			}
			if r.URL.Query().Get("destructive") != "true" {
				writeAPIError(w, http.StatusBadRequest, "destructive_required", "destructive=true expected")
				return
			}
			mergeConfig(cfg, patch)
			fi.version++
			reply(w, map[string]any{})
		})
	})
}

func jsonEq(t *testing.T, want, got string) {
	t.Helper()
	var w, g any
	require.NoError(t, json.Unmarshal([]byte(want), &w))
	require.NoError(t, json.Unmarshal([]byte(got), &g))
	require.Equal(t, w, g)
}

func TestInstanceConfig_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	fi := fakeInstancesOf(f)
	fi.seed(f, "app_1", "ins_1", map[string]any{
		"restrictions": map[string]any{"allowlist": map[string]any{"enabled": false}, "blocklist": map[string]any{"enabled": true}},
		"session":      map[string]any{"lifetime": 604800},
		"unmanaged":    "left alone",
	})
	urn := clerkUrn("InstanceConfig", "c")

	inputs := pm(map[string]property.Value{
		"applicationId": str("app_1"),
		"instanceId":    str("ins_1"),
		"config":        str(`{"session":{"lifetime":3600},"restrictions":{"allowlist":{"enabled":true}}}`),
	})
	// Check marks the config secret; the secret flag then flows to the state.
	checked, err := s.Check(pgp.CheckRequest{Urn: urn, Inputs: inputs})
	require.NoError(t, err)
	require.Empty(t, checked.Failures)
	require.True(t, checked.Inputs.Get("config").Secret(), "config may hold client secrets")
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: checked.Inputs})
	require.NoError(t, err)
	require.Equal(t, "app_1/ins_1", created.ID)
	require.NotEmpty(t, getStr(t, created.Properties, "configVersion"))
	jsonEq(t, `{"session":{"lifetime":3600},"restrictions":{"allowlist":{"enabled":true}}}`, getStr(t, created.Properties, "config"))
	require.True(t, created.Properties.Get("config").Secret(), "config may hold client secrets")
	require.Equal(t, 1, f.callCount("PATCH /platform/applications/app_1/instances/ins_1/config"))
	// Merge semantics: settings that were not named survive.
	require.Equal(t, "left alone", fi.configs["app_1/ins_1"]["unmanaged"])
	require.Equal(t, true, fi.configs["app_1/ins_1"]["restrictions"].(map[string]any)["blocklist"].(map[string]any)["enabled"])

	// Idempotent diff, also when the JSON is reordered and re-spaced.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)
	reordered := inputs.Set("config", str(`{ "restrictions": {"allowlist": {"enabled": true}},
		"session": {"lifetime": 3600} }`))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: reordered, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// A value change updates in place; a different instance replaces.
	changed := inputs.Set("config", str(`{"session":{"lifetime":7200},"restrictions":{"allowlist":{"enabled":true}}}`))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: changed, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, pgp.Update, d.DetailedDiff["config"].Kind)
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs,
		Inputs: inputs.Set("instanceId", str("ins_2"))})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["instanceId"].Kind)

	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: changed, OldInputs: inputs})
	require.NoError(t, err)
	require.NotEqual(t, getStr(t, created.Properties, "configVersion"), getStr(t, upd.Properties, "configVersion"))
	require.Equal(t, float64(7200), mustSession(t, fi.configs["app_1/ins_1"]))

	// Refresh reports only managed keys, and surfaces drift on them.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: changed})
	require.NoError(t, err)
	jsonEq(t, `{"session":{"lifetime":7200},"restrictions":{"allowlist":{"enabled":true}}}`, getStr(t, read.Properties, "config"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: read.Properties, Inputs: changed, OldInputs: read.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)
	fi.seed(f, "app_1", "ins_1", map[string]any{
		"restrictions": map[string]any{"allowlist": map[string]any{"enabled": false}},
		"session":      map[string]any{"lifetime": 7200}, "unmanaged": "changed too",
	})
	drift, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: changed})
	require.NoError(t, err)
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: drift.Properties, Inputs: changed, OldInputs: drift.Inputs})
	require.NoError(t, err)
	require.True(t, d.HasChanges, "drift on a managed key is reported")

	// Import by ID alone adopts the whole configuration, then diffs clean.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, "app_1", getStr(t, imp.Inputs, "applicationId"))
	require.Equal(t, "ins_1", getStr(t, imp.Inputs, "instanceId"))
	jsonEq(t, `{"restrictions":{"allowlist":{"enabled":false}},"session":{"lifetime":7200},"unmanaged":"changed too"}`, getStr(t, imp.Inputs, "config"))
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)
	_, err = s.Read(pgp.ReadRequest{ID: "no-slash", Urn: urn})
	require.ErrorContains(t, err, "<applicationId>/<instanceId>")

	// Lookup function.
	out, err := invoke(t, s, "getInstanceConfig", map[string]property.Value{"applicationId": str("app_1"), "instanceId": str("ins_1")})
	require.NoError(t, err)
	require.Equal(t, "app_1/ins_1", getStr(t, out, "instanceConfigId"))
	jsonEq(t, `{"restrictions":{"allowlist":{"enabled":false}},"session":{"lifetime":7200},"unmanaged":"changed too"}`, getStr(t, out, "config"))
	_, err = invoke(t, s, "getInstanceConfig", map[string]property.Value{"applicationId": str("app_1"), "instanceId": str("ins_missing")})
	require.ErrorContains(t, err, "was not found")

	// Delete only forgets: the instance keeps its configuration and no call is made.
	calls := f.callCount("")
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
	require.Equal(t, calls, f.callCount(""))
	require.Contains(t, fi.configs, "app_1/ins_1")

	// The instance disappeared outside Pulumi.
	delete(fi.configs, "app_1/ins_1")
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: changed})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
}

func mustSession(t *testing.T, cfg map[string]any) any {
	t.Helper()
	sess, ok := cfg["session"].(map[string]any)
	require.True(t, ok)
	return sess["lifetime"]
}

func TestInstanceConfig_InvalidJSONAndDryRun(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	fakeInstancesOf(f).seed(f, "app_1", "ins_1", map[string]any{})
	urn := clerkUrn("InstanceConfig", "c")
	bad := pm(map[string]property.Value{"applicationId": str("app_1"), "instanceId": str("ins_1"), "config": str("{not json")})
	_, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: bad, DryRun: true})
	require.ErrorContains(t, err, "valid JSON")
	_, err = s.Create(pgp.CreateRequest{Urn: urn, Properties: bad})
	require.ErrorContains(t, err, "valid JSON")
	arr := bad.Set("config", str("[1]"))
	_, err = s.Create(pgp.CreateRequest{Urn: urn, Properties: arr})
	require.Error(t, err)

	good := bad.Set("config", str(`{"a":1}`))
	resp, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: good, DryRun: true})
	require.NoError(t, err)
	require.Equal(t, "app_1/ins_1", resp.ID)
	require.Equal(t, 0, f.callCount(""))
}

// InstanceConfig needs the Platform API key, and says so.
func TestInstanceConfig_MissingPlatformKey(t *testing.T) {
	f := startFakeClerk(t)
	for _, v := range []string{"CLERK_API_KEY", "CLERK_SECRET_KEY", "CLERK_PLATFORM_API_KEY", "CLERK_API_URL"} {
		t.Setenv(v, "")
	}
	p, err := New()
	require.NoError(t, err)
	s, err := integration.NewServer(t.Context(), "clerk", semver.MustParse("0.0.1"), integration.WithProvider(p))
	require.NoError(t, err)
	require.NoError(t, s.Configure(pgp.ConfigureRequest{Args: pm(map[string]property.Value{
		"apiKey": str(fakeAPIKey), "apiUrl": str(f.server.URL)})}))
	in := pm(map[string]property.Value{"applicationId": str("a"), "instanceId": str("i"), "config": str("{}")})
	_, err = s.Create(pgp.CreateRequest{Urn: clerkUrn("InstanceConfig", "c"), Properties: in})
	require.ErrorContains(t, err, "clerk:platformApiKey")
	_, err = invoke(t, s, "getInstanceConfig", map[string]property.Value{"applicationId": str("a"), "instanceId": str("i")})
	require.ErrorContains(t, err, "clerk:platformApiKey")
}
