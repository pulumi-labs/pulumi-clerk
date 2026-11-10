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
	"fmt"
	"net/http"
	"net/url"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Machine is a Clerk machine: a non-human identity that authenticates to the
// Backend API with a machine secret key.
type Machine struct{}

// MachineArgs are the inputs of a Machine.
type MachineArgs struct {
	// Name is the machine name.
	Name string `pulumi:"name"`
	// DefaultTokenTtl is the default lifetime of machine tokens, in seconds.
	DefaultTokenTtl *int `pulumi:"defaultTokenTtl,optional"`
}

// MachineState is the stored state of a Machine.
type MachineState struct {
	MachineArgs
	SecretKey                string `pulumi:"secretKey" provider:"secret"`
	EffectiveDefaultTokenTtl int    `pulumi:"effectiveDefaultTokenTtl"`
	CreatedAt                string `pulumi:"createdAt"`
	UpdatedAt                string `pulumi:"updatedAt"`
}

func (r *Machine) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk machine: a non-human identity that authenticates to the Clerk Backend API with a machine secret key.")
}

func (a *MachineArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "Name of the machine.")
	ann.Describe(&a.DefaultTokenTtl, "Default lifetime of the machine's tokens, in seconds. When unset Clerk applies its default, reported in effectiveDefaultTokenTtl.")
}

func (s *MachineState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.SecretKey, "The machine secret key. Secret. Clerk can return it at any time, so it is also recovered when the machine is imported.")
	ann.Describe(&s.EffectiveDefaultTokenTtl, "The default token lifetime in seconds that Clerk is using, including its default.")
	ann.Describe(&s.CreatedAt, "When the machine was created (RFC 3339).")
	ann.Describe(&s.UpdatedAt, "When the machine was last updated (RFC 3339).")
}

var _ infer.CustomResource[MachineArgs, MachineState] = (*Machine)(nil)
var _ infer.CustomUpdate[MachineArgs, MachineState] = (*Machine)(nil)
var _ infer.CustomRead[MachineArgs, MachineState] = (*Machine)(nil)
var _ infer.CustomDelete[MachineState] = (*Machine)(nil)

// machineJSON is the API shape.
type machineJSON struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	DefaultTokenTTL int    `json:"default_token_ttl"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	// SecretKey is only present on the create response.
	SecretKey string `json:"secret_key"`
}

func (j machineJSON) outputs(in MachineArgs, secret string) MachineState {
	return MachineState{
		MachineArgs:              in,
		SecretKey:                secret,
		EffectiveDefaultTokenTtl: j.DefaultTokenTTL,
		CreatedAt:                millisToRFC3339(j.CreatedAt),
		UpdatedAt:                millisToRFC3339(j.UpdatedAt),
	}
}

func machineBody(in MachineArgs) map[string]any {
	body := map[string]any{"name": in.Name}
	if in.DefaultTokenTtl != nil {
		body["default_token_ttl"] = *in.DefaultTokenTtl
	}
	return body
}

func getMachine(ctx context.Context, c *client, id string) (*machineJSON, error) {
	var out machineJSON
	if err := c.do(ctx, http.MethodGet, "/machines/"+id, nil, &out); err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func fetchMachineSecret(ctx context.Context, c *client, id string) (string, error) {
	var out struct {
		Secret string `json:"secret"`
	}
	if err := c.do(ctx, http.MethodGet, "/machines/"+id+"/secret_key", nil, &out); err != nil {
		return "", err
	}
	return out.Secret, nil
}

func (r *Machine) Create(ctx context.Context, req infer.CreateRequest[MachineArgs]) (infer.CreateResponse[MachineState], error) {
	if req.DryRun {
		return infer.CreateResponse[MachineState]{Output: MachineState{MachineArgs: req.Inputs}}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.CreateResponse[MachineState]{}, err
	}
	var out machineJSON
	if err := c.do(ctx, http.MethodPost, "/machines", machineBody(req.Inputs), &out); err != nil {
		return infer.CreateResponse[MachineState]{}, fmt.Errorf("clerk: create machine %q: %w", req.Inputs.Name, err)
	}
	secret := out.SecretKey
	if secret == "" {
		// The machine exists; recover the key or report a partial create so it is not leaked.
		var serr error
		if secret, serr = fetchMachineSecret(ctx, c, out.ID); serr != nil {
			return infer.CreateResponse[MachineState]{ID: out.ID, Output: out.outputs(req.Inputs, "")},
				infer.ResourceInitFailedError{Reasons: []string{fmt.Sprintf("machine %s was created but its secret key could not be read: %v", out.ID, serr)}}
		}
	}
	return infer.CreateResponse[MachineState]{ID: out.ID, Output: out.outputs(req.Inputs, secret)}, nil
}

func (r *Machine) Read(ctx context.Context, req infer.ReadRequest[MachineArgs, MachineState]) (infer.ReadResponse[MachineArgs, MachineState], error) {
	empty := infer.ReadResponse[MachineArgs, MachineState]{}
	c, err := backendClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := getMachine(ctx, c, req.ID)
	if err != nil {
		return empty, fmt.Errorf("clerk: read machine %q: %w", req.ID, err)
	}
	if got == nil {
		if err := notFoundOnImport("machine", req.ID, req.Inputs, req.State); err != nil {
			return empty, err
		}
		return empty, nil
	}
	in := req.Inputs
	imported := in.Name == ""
	in.Name = got.Name
	// Import adopts nothing optional: Clerk's default is in effectiveDefaultTokenTtl (#1563).
	if !imported && in.DefaultTokenTtl != nil {
		v := got.DefaultTokenTTL
		in.DefaultTokenTtl = &v
	}
	secret := req.State.SecretKey
	if secret == "" {
		// Imported (or never captured): Clerk can hand the key out again.
		if secret, err = fetchMachineSecret(ctx, c, req.ID); err != nil {
			return empty, fmt.Errorf("clerk: read secret key of machine %q: %w", req.ID, err)
		}
	}
	return infer.ReadResponse[MachineArgs, MachineState]{ID: got.ID, Inputs: in, State: got.outputs(in, secret)}, nil
}

func (r *Machine) Update(ctx context.Context, req infer.UpdateRequest[MachineArgs, MachineState]) (infer.UpdateResponse[MachineState], error) {
	if req.DryRun {
		st := req.State
		st.MachineArgs = req.Inputs
		return infer.UpdateResponse[MachineState]{Output: st}, nil
	}
	c, err := backendClient(ctx)
	if err != nil {
		return infer.UpdateResponse[MachineState]{}, err
	}
	var out machineJSON
	if err := c.do(ctx, http.MethodPatch, "/machines/"+req.ID, machineBody(req.Inputs), &out); err != nil {
		return infer.UpdateResponse[MachineState]{}, fmt.Errorf("clerk: update machine %q: %w", req.ID, err)
	}
	// The secret key is not returned by update: keep the one we hold.
	return infer.UpdateResponse[MachineState]{Output: out.outputs(req.Inputs, req.State.SecretKey)}, nil
}

func (r *Machine) Delete(ctx context.Context, req infer.DeleteRequest[MachineState]) (infer.DeleteResponse, error) {
	c, err := backendClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/machines/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete machine %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetMachine looks up a machine by ID or name.
type GetMachine struct{}

// GetMachineArgs selects the machine; set exactly one of the fields.
type GetMachineArgs struct {
	ID   *string `pulumi:"machineId,optional"`
	Name *string `pulumi:"name,optional"`
}

// GetMachineResult is the looked-up machine. It never includes the secret key.
type GetMachineResult struct {
	ID              string `pulumi:"machineId"`
	Name            string `pulumi:"name"`
	DefaultTokenTtl int    `pulumi:"defaultTokenTtl"`
	CreatedAt       string `pulumi:"createdAt"`
	UpdatedAt       string `pulumi:"updatedAt"`
}

func (f *GetMachine) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk machine by ID or by exact name. The secret key is not returned.")
}

func (a *GetMachineArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The machine ID. Set either machineId or name.")
	ann.Describe(&a.Name, "The machine name. Set either machineId or name.")
}

func (f *GetMachine) Invoke(ctx context.Context, req infer.FunctionRequest[GetMachineArgs]) (infer.FunctionResponse[GetMachineResult], error) {
	out := infer.FunctionResponse[GetMachineResult]{}
	id, name := derefStr(req.Input.ID), derefStr(req.Input.Name)
	if (id == "") == (name == "") {
		return out, fmt.Errorf("clerk: getMachine needs exactly one of machineId or name")
	}
	c, err := backendClient(ctx)
	if err != nil {
		return out, err
	}
	var got *machineJSON
	if id != "" {
		got, err = getMachine(ctx, c, id)
	} else {
		var items []machineJSON
		items, err = listAll[machineJSON](ctx, c, "/machines", url.Values{"query": {name}})
		for i := range items {
			if items[i].Name == name {
				got = &items[i]
				break
			}
		}
	}
	if err != nil {
		return out, lookupErr("machine", id+name, err)
	}
	if got == nil {
		return out, lookupErr("machine", fmt.Sprintf("%q", id+name), &apiError{Status: http.StatusNotFound})
	}
	out.Output = GetMachineResult{ID: got.ID, Name: got.Name, DefaultTokenTtl: got.DefaultTokenTTL,
		CreatedAt: millisToRFC3339(got.CreatedAt), UpdatedAt: millisToRFC3339(got.UpdatedAt)}
	return out, nil
}
