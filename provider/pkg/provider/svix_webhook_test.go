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
	"fmt"
	"net/http"
	"testing"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// fakeSvix is the Svix app of the fake instance.
type fakeSvix struct {
	exists bool
	urls   int
}

func fakeSvixOf(f *fakeClerk) *fakeSvix { return f.state["svix"].(*fakeSvix) }

func init() {
	fakeRegistrars = append(fakeRegistrars, func(f *fakeClerk) {
		sv := &fakeSvix{}
		f.state["svix"] = sv
		url := func() map[string]any {
			sv.urls++
			return map[string]any{"object": "svix_url", "svix_url": fmt.Sprintf("https://app.svix.com/login#key=%d", sv.urls)}
		}
		f.mux.HandleFunc("POST /webhooks/svix", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if sv.exists {
				writeAPIError(w, http.StatusConflict, "svix_already_enabled", "Svix is already enabled")
				return
			}
			sv.exists = true
			writeJSON(w, http.StatusOK, url())
		})
		f.mux.HandleFunc("POST /webhooks/svix_url", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if !sv.exists {
				notFound(w, "svix app")
				return
			}
			writeJSON(w, http.StatusOK, url())
		})
		f.mux.HandleFunc("DELETE /webhooks/svix", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if !sv.exists {
				notFound(w, "svix app")
				return
			}
			sv.exists = false
			writeJSON(w, http.StatusOK, map[string]any{"object": "svix_url", "svix_url": ""})
		})
	})
}

func TestSvixWebhook_Lifecycle(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	urn := clerkUrn("SvixWebhook", "w")
	inputs := pm(map[string]property.Value{})

	created, err := s.Create(pgp.CreateRequest{Urn: urn, Properties: inputs})
	require.NoError(t, err)
	require.Equal(t, "svix_webhook", created.ID)
	require.Contains(t, getStr(t, created.Properties, "svixUrl"), "https://app.svix.com/")
	require.True(t, fakeSvixOf(f).exists)

	// A second one is refused by Clerk.
	_, err = s.Create(pgp.CreateRequest{Urn: clerkUrn("SvixWebhook", "w2"), Properties: inputs})
	require.ErrorContains(t, err, "already enabled")

	// Idempotent diff: no inputs, nothing to change.
	d, err := s.Diff(pgp.DiffRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)

	// Refresh by ID, and import by ID alone: both fetch a fresh dashboard URL.
	read, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Equal(t, "svix_webhook", read.ID)
	require.NotEqual(t, getStr(t, created.Properties, "svixUrl"), getStr(t, read.Properties, "svixUrl"))
	imp, err := s.Read(pgp.ReadRequest{ID: "svix_webhook", Urn: urn})
	require.NoError(t, err)
	require.NotEmpty(t, getStr(t, imp.Properties, "svixUrl"))
	d, err = s.Diff(pgp.DiffRequest{ID: imp.ID, Urn: urn, State: imp.Properties, Inputs: imp.Inputs, OldInputs: imp.Inputs})
	require.NoError(t, err)
	requireNoChanges(t, d)
	_, err = s.Read(pgp.ReadRequest{ID: "something-else", Urn: urn})
	require.ErrorContains(t, err, "fixed ID")

	// Update is a no-op.
	upd, err := s.Update(pgp.UpdateRequest{ID: created.ID, Urn: urn, State: created.Properties, Inputs: inputs, OldInputs: inputs})
	require.NoError(t, err)
	require.Equal(t, getStr(t, created.Properties, "svixUrl"), getStr(t, upd.Properties, "svixUrl"))

	// Lookup function.
	out, err := invoke(t, s, "getSvixWebhook", map[string]property.Value{})
	require.NoError(t, err)
	require.Equal(t, "svix_webhook", getStr(t, out, "svixWebhookId"))
	require.NotEmpty(t, getStr(t, out, "svixUrl"))

	// Delete, then the integration is gone: Read is empty, Delete is quiet,
	// and the lookup says so.
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	require.False(t, fakeSvixOf(f).exists)
	gone, err := s.Read(pgp.ReadRequest{ID: created.ID, Urn: urn, Properties: created.Properties, Inputs: inputs})
	require.NoError(t, err)
	require.Empty(t, gone.ID)
	require.NoError(t, s.Delete(pgp.DeleteRequest{ID: created.ID, Urn: urn, Properties: created.Properties}))
	_, err = invoke(t, s, "getSvixWebhook", map[string]property.Value{})
	require.ErrorContains(t, err, "was not found")
}

func TestSvixWebhook_DryRunDoesNotCallAPI(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	resp, err := s.Create(pgp.CreateRequest{Urn: clerkUrn("SvixWebhook", "w"), Properties: pm(map[string]property.Value{}), DryRun: true})
	require.NoError(t, err)
	require.Empty(t, resp.ID)
	require.Equal(t, 0, f.callCount(""))
}
