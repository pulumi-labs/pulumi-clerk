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
	"encoding/base64"
	"io"
	"net/http"
	"testing"

	"github.com/blang/semver"
	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// fakePlatformApps backs the /platform/applications routes.
type fakePlatformApps struct {
	apps    map[string]fakeObj
	images  map[string]string // "<app>/<logo|favicon>" -> uploaded bytes
	domains map[string]map[string]fakeObj
	failUp  bool
}

func platformApps(f *fakeClerk) *fakePlatformApps { return f.state["platformApps"].(*fakePlatformApps) }

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		p := &fakePlatformApps{apps: map[string]fakeObj{}, images: map[string]string{}, domains: map[string]map[string]fakeObj{}}
		f.state["platformApps"] = p
		f.mux.HandleFunc("POST /platform/applications", func(w http.ResponseWriter, r *http.Request) {
			body := readBody(r)
			f.mu.Lock()
			defer f.mu.Unlock()
			id := f.nextID("app")
			envs, _ := body["environment_types"].([]any)
			if len(envs) == 0 {
				envs = []any{"development", "production"}
			}
			insts := []any{}
			for _, e := range envs {
				iid := f.nextID("ins")
				insts = append(insts, map[string]any{"instance_id": iid, "environment_type": e, "secret_key": "sk_" + iid, "publishable_key": "pk_" + iid})
			}
			p.apps[id] = fakeObj{"application_id": id, "instances": insts}
			writeJSON(w, http.StatusOK, p.apps[id])
		})
		f.mux.HandleFunc("GET /platform/applications/{id}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			a, ok := p.apps[r.PathValue("id")]
			if !ok {
				notFound(w, "application")
				return
			}
			writeJSON(w, http.StatusOK, a)
		})
		f.mux.HandleFunc("PATCH /platform/applications/{id}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			a, ok := p.apps[r.PathValue("id")]
			if !ok {
				notFound(w, "application")
				return
			}
			a["name"] = readBody(r)["name"]
			// The update answer omits secret keys.
			stripped := []any{}
			for _, i := range a["instances"].([]any) {
				m := i.(map[string]any)
				stripped = append(stripped, map[string]any{"instance_id": m["instance_id"], "environment_type": m["environment_type"]})
			}
			writeJSON(w, http.StatusOK, map[string]any{"application_id": a["application_id"], "instances": stripped})
		})
		f.mux.HandleFunc("DELETE /platform/applications/{id}", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			id := r.PathValue("id")
			if _, ok := p.apps[id]; !ok {
				notFound(w, "application")
				return
			}
			delete(p.apps, id)
			writeJSON(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
		})
		for _, kind := range []string{"logo", "favicon"} {
			kind := kind
			f.mux.HandleFunc("POST /platform/applications/{id}/"+kind, func(w http.ResponseWriter, r *http.Request) {
				f.mu.Lock()
				defer f.mu.Unlock()
				if p.failUp {
					writeAPIError(w, http.StatusUnprocessableEntity, "upload_failed", "bad image")
					return
				}
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					writeAPIError(w, http.StatusBadRequest, "bad_multipart", err.Error())
					return
				}
				file, _, err := r.FormFile("file")
				if err != nil {
					writeAPIError(w, http.StatusBadRequest, "no_file", err.Error())
					return
				}
				data, _ := io.ReadAll(file)
				p.images[r.PathValue("id")+"/"+kind] = string(data)
				writeJSON(w, http.StatusOK, map[string]any{})
			})
			f.mux.HandleFunc("DELETE /platform/applications/{id}/"+kind, func(w http.ResponseWriter, r *http.Request) {
				f.mu.Lock()
				defer f.mu.Unlock()
				delete(p.images, r.PathValue("id")+"/"+kind)
				writeJSON(w, http.StatusOK, map[string]any{})
			})
		}
	})
}

func upSecret(t *testing.T, s integration.Server, urn resource.URN, in property.Map) bool {
	t.Helper()
	ch, err := s.Check(pgp.CheckRequest{Urn: urn, Inputs: in})
	require.NoError(t, err)
	return ch.Inputs.Get("logo").Secret()
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func TestApplication_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	p := platformApps(f)
	urn := clerkUrn("Application", "app")

	// A dry run does not touch the API.
	inputs := pm(map[string]property.Value{"name": str("Acme"), "logo": str(b64("PNGDATA")), "environmentTypes": strArray("development")})
	pre, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs, DryRun: true})
	require.NoError(t, err)
	require.Empty(t, pre.ID)
	require.Zero(t, f.callCount("POST /platform/applications"))

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "PNGDATA", p.images[created.ID+"/logo"])
	insts, ok := created.Properties.GetOk("instances")
	require.True(t, ok)
	require.Len(t, insts.AsArray().AsSlice(), 1)
	inst := insts.AsArray().AsSlice()[0].AsMap()
	require.True(t, insts.Secret(), "instances carry secret keys and must be secret")
	require.Equal(t, "sk_"+getStr(t, inst, "instanceId"), inst.Get("secretKey").AsString())

	require.True(t, upSecret(t, s, urn, inputs), "logo input must be secret after Check")

	// Idempotent diff.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// create-only inputs replace; name updates in place.
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs,
		Inputs: pm(map[string]property.Value{"name": str("Acme"), "logo": str(b64("PNGDATA")), "environmentTypes": strArray("production")})})
	require.NoError(t, err)
	require.Equal(t, pgp.UpdateReplace, d.DetailedDiff["environmentTypes"].Kind)
	renamed := pm(map[string]property.Value{"name": str("Acme 2"), "logo": str(b64("NEWLOGO")), "favicon": str(b64("ICO")), "environmentTypes": strArray("development")})
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs, Inputs: renamed})
	require.NoError(t, err)
	require.True(t, d.HasChanges)
	require.Equal(t, pgp.Update, d.DetailedDiff["name"].Kind)

	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, OldInputs: inputs, Inputs: renamed})
	require.NoError(t, err)
	require.Equal(t, "NEWLOGO", p.images[created.ID+"/logo"])
	require.Equal(t, "ICO", p.images[created.ID+"/favicon"])
	// Secret keys survive an update whose answer omits them.
	u := upd.Properties.Get("instances").AsArray().AsSlice()[0].AsMap()
	require.Equal(t, "sk_"+getStr(t, u, "instanceId"), u.Get("secretKey").AsString())

	// Dropping the images deletes them.
	bare := pm(map[string]property.Value{"name": str("Acme 2"), "environmentTypes": strArray("development")})
	upd2, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: upd.Properties, OldInputs: renamed, Inputs: bare})
	require.NoError(t, err)
	require.Empty(t, p.images)
	_ = upd2

	// Refresh keeps configured values and reports the keys.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: renamed})
	require.NoError(t, err)
	require.Equal(t, "Acme 2", getStr(t, read.Inputs, "name"))
	ri := read.Properties.Get("instances").AsArray().AsSlice()[0].AsMap()
	require.True(t, read.Properties.Get("instances").Secret())
	_ = ri

	// Import by ID alone, then no changes against what Read returned.
	imp, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn})
	require.NoError(t, err)
	require.Equal(t, created.ID, imp.ID)
	require.Len(t, imp.Properties.Get("instances").AsArray().AsSlice(), 1)
	d, err = s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Lookup function.
	out, err := invoke(t, s, "getApplication", map[string]property.Value{"applicationId": str(created.ID)})
	require.NoError(t, err)
	require.Equal(t, created.ID, getStr(t, out, "applicationId"))
	require.Len(t, out.Get("instances").AsArray().AsSlice(), 1)
	require.True(t, out.Get("instances").Secret(), "lookup result must keep keys secret")
	_, err = invoke(t, s, "getApplication", map[string]property.Value{"applicationId": str("app_missing")})
	require.ErrorContains(t, err, "was not found")

	// Deleted outside Pulumi.
	f.mu.Lock()
	delete(p.apps, created.ID)
	f.mu.Unlock()
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: upd.Properties, Inputs: renamed})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: upd.Properties}))
}

func TestApplication_Delete(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("Application", "app")
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"name": str("Acme")})})
	require.NoError(t, err)
	require.Len(t, created.Properties.Get("instances").AsArray().AsSlice(), 2)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.Empty(t, platformApps(f).apps)
}

// A failed image upload after the application exists keeps the ID in state.
func TestApplication_PartialCreate(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	platformApps(f).failUp = true
	urn := clerkUrn("Application", "app")
	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: pm(map[string]property.Value{"name": str("Acme"), "favicon": str(b64("x"))})})
	require.Error(t, err)
	require.NotEmpty(t, created.ID, "the created application must be tracked")
	require.Len(t, platformApps(f).apps, 1)
}

func TestApplication_BadBase64(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	_, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("Application", "app"), Properties: pm(map[string]property.Value{"name": str("Acme"), "logo": str("!!not base64!!")})})
	require.ErrorContains(t, err, "not valid base64")
	require.Zero(t, f.callCount("POST /platform/applications"))
}

// Without a platform key the error says how to set it.
func TestPlatformResources_MissingPlatformKey(t *testing.T) {
	for _, v := range []string{"CLERK_API_KEY", "CLERK_SECRET_KEY", "CLERK_PLATFORM_API_KEY", "CLERK_API_URL"} {
		t.Setenv(v, "")
	}
	p, err := New()
	require.NoError(t, err)
	s, err := integration.NewServer(t.Context(), "clerk", semver.MustParse("0.0.1"), integration.WithProvider(p))
	require.NoError(t, err)
	require.NoError(t, s.Configure(pgp.ConfigureRequest{Args: pm(map[string]property.Value{"apiKey": str(fakeAPIKey)})}))
	_, err = s.Create(pgp.CreateRequest{Urn: clerkUrn("Application", "a"), Properties: pm(map[string]property.Value{"name": str("x")})})
	require.ErrorContains(t, err, "clerk:platformApiKey")
	_, err = s.Create(pgp.CreateRequest{Urn: clerkUrn("Domain", "d"), Properties: pm(map[string]property.Value{"applicationId": str("app_1"), "name": str("x.example.com")})})
	require.ErrorContains(t, err, "clerk:platformApiKey")
}
