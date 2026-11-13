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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClerk is an in-memory stand-in for the Clerk Backend and Platform APIs
// served over HTTP. Routes use net/http's method-and-wildcard patterns. Each
// resource file registers its routes from an init function by appending to
// fakeRegistrars.
type fakeClerk struct {
	mu     sync.Mutex
	seq    int
	mux    *http.ServeMux
	server *httptest.Server
	// calls records "METHOD /path" for every request that reached a handler.
	calls []string
	// Collections by path prefix, for assertions and for seeding objects.
	collections map[string]*fakeCollection
	// Extra per-test state hooks (for resources with bespoke endpoints).
	state map[string]any
}

const (
	fakeAPIKey         = "sk_test_fake"
	fakePlatformAPIKey = "ak_fake"
)

var fakeRegistrars []func(*fakeClerk)

func startFakeClerk(t *testing.T) *fakeClerk {
	t.Helper()
	f := &fakeClerk{mux: http.NewServeMux(), collections: map[string]*fakeCollection{}, state: map[string]any{}}
	for _, reg := range fakeRegistrars {
		reg(f)
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if got != "Bearer "+fakeAPIKey && got != "Bearer "+fakePlatformAPIKey {
			writeAPIError(w, http.StatusUnauthorized, "authentication_invalid", "Invalid authentication")
			return
		}
		f.mu.Lock()
		f.calls = append(f.calls, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		f.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeClerk) nextID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s_%04d", prefix, f.seq)
}

func (f *fakeClerk) callCount(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// fakeCodedError is a hook error that carries its own Clerk error code.
type fakeCodedError struct{ code, msg string }

func (e *fakeCodedError) Error() string { return e.msg }

// errDeprecatedParam is what Clerk answers when a deprecated parameter is sent.
var errDeprecatedParam error = &fakeCodedError{code: "form_param_deprecated", msg: "public_metadata is deprecated and is no longer a valid parameter for this request. Use PATCH /v1/organizations/{organization_id}/metadata to merge updates into existing metadata, or PUT /v1/organizations/{organization_id}/metadata to replace a metadata field entirely."}

func writeAPIError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"errors": []map[string]any{{"code": code, "message": msg, "long_message": msg}}})
}

func notFound(w http.ResponseWriter, what string) {
	writeAPIError(w, http.StatusNotFound, "resource_not_found", what+" not found")
}

func readBody(r *http.Request) map[string]any {
	out := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&out)
	return out
}

// fakeObj is a JSON object the fake stores and returns verbatim.
type fakeObj map[string]any

func (o fakeObj) str(k string) string {
	s, _ := o[k].(string)
	return s
}

// fakeCollection is a generic REST collection: POST creates, GET lists,
// GET {id} reads (when ByID), PATCH {id} updates (when Patch), DELETE {id}
// deletes. Hooks customise validation and server-computed fields.
type fakeCollection struct {
	f      *fakeClerk
	Path   string
	Prefix string
	ByID   bool
	Patch  bool
	// Object is the value of the "object" field on returned objects.
	Object string
	// Wrap, when set, nests the list under this key instead of "data"
	// (and omits it for a bare-array list when Bare is true).
	Bare bool
	// OnCreate may reject (return an error) or fill server fields.
	OnCreate func(obj fakeObj, body map[string]any) error
	// OnUpdate may reject or fill server fields; body is the merge patch.
	OnUpdate func(obj fakeObj, body map[string]any) error
	Objs     map[string]fakeObj
	Order    []string
}

func (f *fakeClerk) collection(c *fakeCollection) *fakeCollection {
	c.f = f
	c.Objs = map[string]fakeObj{}
	f.collections[c.Path] = c
	f.mux.HandleFunc("POST "+c.Path, c.create)
	f.mux.HandleFunc("GET "+c.Path, c.list)
	if c.ByID {
		f.mux.HandleFunc("GET "+c.Path+"/{id}", c.get)
	}
	if c.Patch {
		f.mux.HandleFunc("PATCH "+c.Path+"/{id}", c.update)
	}
	f.mux.HandleFunc("DELETE "+c.Path+"/{id}", c.remove)
	return c
}

func (c *fakeCollection) create(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	obj := fakeObj{}
	for k, v := range body {
		obj[k] = v
	}
	obj["id"] = c.f.nextID(c.Prefix)
	if c.Object != "" {
		obj["object"] = c.Object
	}
	now := time.Now().UnixMilli()
	obj["created_at"], obj["updated_at"] = now, now
	if c.OnCreate != nil {
		if err := c.OnCreate(obj, body); err != nil {
			writeAPIError(w, http.StatusUnprocessableEntity, "form_param_invalid", err.Error())
			return
		}
	}
	c.Objs[obj.str("id")] = obj
	c.Order = append(c.Order, obj.str("id"))
	writeJSON(w, http.StatusOK, obj)
}

func (c *fakeCollection) list(w http.ResponseWriter, r *http.Request) {
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	items := []fakeObj{}
	for _, id := range c.Order {
		if o, ok := c.Objs[id]; ok {
			items = append(items, o)
		}
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	total := len(items)
	if offset > len(items) {
		offset = len(items)
	}
	items = items[offset:]
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	if c.Bare {
		writeJSON(w, http.StatusOK, items)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "total_count": total})
}

func (c *fakeCollection) get(w http.ResponseWriter, r *http.Request) {
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	o, ok := c.Objs[r.PathValue("id")]
	if !ok {
		notFound(w, c.Object)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (c *fakeCollection) update(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	o, ok := c.Objs[r.PathValue("id")]
	if !ok {
		notFound(w, c.Object)
		return
	}
	for k, v := range body {
		o[k] = v
	}
	o["updated_at"] = time.Now().UnixMilli()
	if c.OnUpdate != nil {
		if err := c.OnUpdate(o, body); err != nil {
			code := "form_param_invalid"
			var coded *fakeCodedError
			if errors.As(err, &coded) {
				code = coded.code
			}
			writeAPIError(w, http.StatusUnprocessableEntity, code, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, o)
}

func (c *fakeCollection) remove(w http.ResponseWriter, r *http.Request) {
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	id := r.PathValue("id")
	if _, ok := c.Objs[id]; !ok {
		notFound(w, c.Object)
		return
	}
	delete(c.Objs, id)
	writeJSON(w, http.StatusOK, map[string]any{"object": "deleted_object", "id": id, "deleted": true})
}

// seed inserts an object as if created outside Pulumi and returns its ID.
func (c *fakeCollection) seed(obj fakeObj) string {
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	id := obj.str("id")
	if id == "" {
		id = c.f.nextID(c.Prefix)
		obj["id"] = id
	}
	if c.Object != "" {
		obj["object"] = c.Object
	}
	now := time.Now().UnixMilli()
	if _, ok := obj["created_at"]; !ok {
		obj["created_at"] = now
	}
	if _, ok := obj["updated_at"]; !ok {
		obj["updated_at"] = now
	}
	c.Objs[id] = obj
	c.Order = append(c.Order, id)
	return id
}

// dropOutside deletes an object as if removed outside Pulumi.
func (c *fakeCollection) dropOutside(id string) {
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	delete(c.Objs, id)
}
