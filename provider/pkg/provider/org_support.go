// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"net/url"
	"sort"
	"strings"

	pgp "github.com/pulumi/pulumi-go-provider"
)

// This file holds helpers shared by the organization-family resources
// (Organization, OrganizationDomain, OrganizationMembership,
// OrganizationPermission, OrganizationRole and RoleSet).

// canonJSON re-encodes a JSON document with sorted object keys and no
// insignificant whitespace, so documents that differ only in key order or
// spacing compare equal. Text that is not valid JSON is returned unchanged.
func canonJSON(s string) string {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return s
	}
	return string(b)
}

// emptyMeta reports whether a metadata value from the API carries nothing.
func emptyMeta(raw json.RawMessage) bool {
	t := strings.TrimSpace(string(raw))
	return t == "" || t == "null" || t == "{}"
}

// canonOrEmpty canonicalises optional metadata; unset equals "{}".
func canonOrEmpty(p *string) string {
	if p == nil {
		return "{}"
	}
	return canonJSON(*p)
}

// keepJSON turns metadata returned by the API into the string stored in
// state. The previous string is kept when it is semantically identical, so a
// user's formatting never shows up as drift.
func keepJSON(prev *string, server json.RawMessage) *string {
	if emptyMeta(server) {
		if prev != nil && canonJSON(*prev) == "{}" {
			return prev
		}
		return nil
	}
	if prev != nil && canonJSON(*prev) == canonJSON(string(server)) {
		return prev
	}
	return strPtr(canonJSON(string(server)))
}

// keepStr keeps the previous optional string when it means the same as the
// server's (nil and "" are the same), otherwise adopts the server value.
func keepStr(prev, server *string) *string {
	if derefStr(prev) == derefStr(server) {
		if prev != nil {
			return prev
		}
	}
	if server != nil && *server == "" && prev == nil {
		return nil
	}
	return server
}

// sameSet reports whether two lists hold the same elements in any order.
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// keepOrder returns the previous list when the server holds the same
// elements (preserving the user's order), otherwise the server's list.
func keepOrder(prev, server []string) []string {
	if prev != nil && sameSet(prev, server) {
		return prev
	}
	return server
}

// diffAcc accumulates a hand-built diff.
type diffAcc struct{ resp pgp.DiffResponse }

func (d *diffAcc) mark(name string, kind pgp.DiffKind) {
	if d.resp.DetailedDiff == nil {
		d.resp.DetailedDiff = map[string]pgp.PropertyDiff{}
	}
	d.resp.HasChanges = true
	d.resp.DetailedDiff[name] = pgp.PropertyDiff{Kind: kind, InputDiff: true}
}

func (d *diffAcc) update(name string)  { d.mark(name, pgp.Update) }
func (d *diffAcc) replace(name string) { d.mark(name, pgp.UpdateReplace) }

// splitID splits a composite "<parentId>/<childId>" resource ID.
func splitID(kind, parent, id string) (string, string, error) {
	p, c, ok := strings.Cut(id, "/")
	if !ok || p == "" || c == "" || strings.Contains(c, "/") {
		return "", "", fmt.Errorf("clerk: invalid %s ID %q: expected <%s>/<id>", kind, id, parent)
	}
	return p, c, nil
}

func esc(s string) string { return url.PathEscape(s) }

// rawJSON is a request-body value that is already JSON text.
func rawJSON(s string) json.RawMessage { return json.RawMessage(s) }
