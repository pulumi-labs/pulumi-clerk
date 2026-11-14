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
	"context"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// An import adopts an optional input only when Clerk's live value differs from the default
// (adoptIfNotDefault), so the imported state holds no value for an input that has Clerk's default. A
// program that then declares that default must plan no change (#1801 and the report in #2012): infer's
// default diff would see an unset input gaining a value. These Diffs treat an unset input and Clerk's
// default as the same value, in both directions, and keep the default diff's kinds for everything else.

// defaultInputKind is the diff kind of an optional input, or false when the two sides mean the same thing:
// the same value, or one side unset and the other Clerk's default.
func defaultInputKind[T comparable](old, cur *T, def T) (pgp.DiffKind, bool) {
	val := func(p *T) T {
		if p == nil {
			return def
		}
		return *p
	}
	if val(old) == val(cur) {
		return pgp.Update, false
	}
	switch {
	case old == nil:
		return pgp.Add, true
	case cur == nil:
		return pgp.Delete, true
	}
	return pgp.Update, true
}

// plainInputKind is the diff kind of an optional input that has no Clerk default.
func plainInputKind[T comparable](old, cur *T) (pgp.DiffKind, bool) {
	switch {
	case old == nil && cur == nil:
		return pgp.Update, false
	case old == nil:
		return pgp.Add, true
	case cur == nil:
		return pgp.Delete, true
	case *old != *cur:
		return pgp.Update, true
	}
	return pgp.Update, false
}

type inputDiff map[string]pgp.PropertyDiff

func (d inputDiff) set(key string, kind pgp.DiffKind, changed bool) {
	if changed {
		d[key] = pgp.PropertyDiff{Kind: kind, InputDiff: true}
	}
}

func (d inputDiff) response() infer.DiffResponse {
	return infer.DiffResponse{HasChanges: len(d) > 0, DetailedDiff: d}
}

var _ infer.CustomDiff[MachineArgs, MachineState] = (*Machine)(nil)

// Diff updates in place; an unset defaultTokenTtl and Clerk's default are the same.
func (r *Machine) Diff(_ context.Context, req infer.DiffRequest[MachineArgs, MachineState]) (infer.DiffResponse, error) {
	in, st := req.Inputs, req.State
	d := inputDiff{}
	if in.Name != st.Name {
		d.set("name", pgp.Update, true)
	}
	k, ch := defaultInputKind(st.DefaultTokenTtl, in.DefaultTokenTtl, clerkDefaultMachineTokenTTL)
	d.set("defaultTokenTtl", k, ch)
	return d.response(), nil
}

var _ infer.CustomDiff[JwtTemplateArgs, JwtTemplateState] = (*JwtTemplate)(nil)

// Diff updates in place; an unset lifetime, allowedClockSkew, customSigningKey and signingAlgorithm are
// the same as Clerk's defaults. The signing key has no default.
func (r *JwtTemplate) Diff(_ context.Context, req infer.DiffRequest[JwtTemplateArgs, JwtTemplateState]) (infer.DiffResponse, error) {
	in, st := req.Inputs, req.State
	d := inputDiff{}
	if in.Name != st.Name {
		d.set("name", pgp.Update, true)
	}
	if in.Claims != st.Claims {
		d.set("claims", pgp.Update, true)
	}
	k, ch := defaultInputKind(st.Lifetime, in.Lifetime, clerkDefaultJwtLifetime)
	d.set("lifetime", k, ch)
	k, ch = defaultInputKind(st.AllowedClockSkew, in.AllowedClockSkew, clerkDefaultJwtClockSkew)
	d.set("allowedClockSkew", k, ch)
	k, ch = defaultInputKind(st.CustomSigningKey, in.CustomSigningKey, clerkDefaultJwtCustomSigningKey)
	d.set("customSigningKey", k, ch)
	k, ch = plainInputKind(st.SigningKey, in.SigningKey)
	d.set("signingKey", k, ch)
	k, ch = defaultInputKind(st.SigningAlgorithm, in.SigningAlgorithm, clerkDefaultJwtSigningAlgorithm)
	d.set("signingAlgorithm", k, ch)
	return d.response(), nil
}

var _ infer.CustomDiff[ApplicationSettingsArgs, ApplicationSettingsState] = (*ApplicationSettings)(nil)

// Diff updates in place; an unset maxAllowedMemberships, adminDeleteEnabled and domainsEnabled are the same
// as Clerk's defaults.
func (r *ApplicationSettings) Diff(_ context.Context, req infer.DiffRequest[ApplicationSettingsArgs, ApplicationSettingsState]) (infer.DiffResponse, error) {
	in, st := req.Inputs, req.State
	d := inputDiff{}
	if in.EnableOrganizations != st.EnableOrganizations {
		d.set("enableOrganizations", pgp.Update, true)
	}
	k, ch := defaultInputKind(st.MaxAllowedMemberships, in.MaxAllowedMemberships, clerkDefaultMaxAllowedMemberships)
	d.set("maxAllowedMemberships", k, ch)
	k, ch = defaultInputKind(st.AdminDeleteEnabled, in.AdminDeleteEnabled, clerkDefaultAdminDeleteEnabled)
	d.set("adminDeleteEnabled", k, ch)
	k, ch = defaultInputKind(st.DomainsEnabled, in.DomainsEnabled, clerkDefaultDomainsEnabled)
	d.set("domainsEnabled", k, ch)
	return d.response(), nil
}
