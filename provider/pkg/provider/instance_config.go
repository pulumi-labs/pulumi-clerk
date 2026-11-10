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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// InstanceConfig manages a subset of the configuration of a Clerk instance
// through the Platform API (beta).
type InstanceConfig struct{}

// InstanceConfigArgs are the inputs of an InstanceConfig.
type InstanceConfigArgs struct {
	ApplicationID string `pulumi:"applicationId" provider:"replaceOnChanges"`
	InstanceID    string `pulumi:"instanceId" provider:"replaceOnChanges"`
	Config        string `pulumi:"config" provider:"secret"`
}

// InstanceConfigState is the stored state of an InstanceConfig.
type InstanceConfigState struct {
	InstanceConfigArgs
	ConfigVersion string `pulumi:"configVersion"`
}

func (r *InstanceConfig) Annotate(a infer.Annotator) {
	a.Describe(&r, "Settings of a Clerk instance, managed through the Platform API (beta, needs the platform API key). Only the keys present in config are managed: other settings of the instance are left alone and are not reported as drift. Singleton per instance: deleting the resource only removes it from Pulumi state and does not reset the instance configuration. Import with the ID <applicationId>/<instanceId>; the imported config is the whole instance configuration.")
}

func (a *InstanceConfigArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ApplicationID, "The ID of the Clerk application. Changing this forces a new resource.")
	ann.Describe(&a.InstanceID, "The ID of the Clerk instance. Changing this forces a new resource.")
	ann.Describe(&a.Config, "JSON object of instance configuration keys and values, for example from JSON.stringify. Key order does not matter. It is a secret because instance configuration can contain provider client secrets.")
}

func (s *InstanceConfigState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.ConfigVersion, "The version of the instance configuration, used by Clerk for optimistic concurrency.")
}

var _ infer.CustomResource[InstanceConfigArgs, InstanceConfigState] = (*InstanceConfig)(nil)
var _ infer.CustomUpdate[InstanceConfigArgs, InstanceConfigState] = (*InstanceConfig)(nil)
var _ infer.CustomRead[InstanceConfigArgs, InstanceConfigState] = (*InstanceConfig)(nil)
var _ infer.CustomDelete[InstanceConfigState] = (*InstanceConfig)(nil)
var _ infer.CustomDiff[InstanceConfigArgs, InstanceConfigState] = (*InstanceConfig)(nil)

// parseConfigObject decodes a JSON object, keeping numbers exact.
func parseConfigObject(s string) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("config is not a valid JSON object: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("config must be a JSON object, not null")
	}
	return m, nil
}

// canonicalConfigJSON renders a config object with sorted keys and no
// insignificant whitespace.
func canonicalConfigJSON(m map[string]any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// normalizeConfigJSON canonicalises a JSON config string so that key order and
// whitespace never show up as a diff.
func normalizeConfigJSON(s string) (string, error) {
	m, err := parseConfigObject(s)
	if err != nil {
		return "", err
	}
	return canonicalConfigJSON(m)
}

// filterConfigToManaged keeps only the keys of api that managed also has,
// recursing through nested objects, so unmanaged settings are not drift.
func filterConfigToManaged(managed, api map[string]any) map[string]any {
	out := map[string]any{}
	for k, mv := range managed {
		av, ok := api[k]
		if !ok {
			continue
		}
		mm, mIsMap := mv.(map[string]any)
		am, aIsMap := av.(map[string]any)
		if mIsMap && aIsMap {
			out[k] = filterConfigToManaged(mm, am)
		} else {
			out[k] = av
		}
	}
	return out
}

func instanceConfigPath(appID, instID string) string {
	return "/platform/applications/" + appID + "/instances/" + instID + "/config"
}

// instanceConfigID joins the application and instance IDs.
func instanceConfigID(appID, instID string) string { return appID + "/" + instID }

// getInstanceConfigRaw reads the whole configuration and its version.
func getInstanceConfigRaw(ctx context.Context, c *client, appID, instID string) (map[string]any, string, error) {
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, instanceConfigPath(appID, instID), nil, &raw); err != nil {
		return nil, "", err
	}
	if raw == nil {
		raw = map[string]any{}
	}
	version, _ := raw["config_version"].(string)
	delete(raw, "config_version")
	return raw, version, nil
}

// putInstanceConfig applies the managed keys; destructive lets null values
// remove settings, as the Terraform provider did.
func putInstanceConfig(ctx context.Context, c *client, appID, instID string, config map[string]any) (string, error) {
	var raw map[string]any
	if err := c.do(ctx, http.MethodPatch, instanceConfigPath(appID, instID)+"?destructive=true", config, &raw); err != nil {
		return "", err
	}
	version, _ := raw["config_version"].(string)
	return version, nil
}

// Diff compares the config canonically, so reordered keys are not a change,
// and replaces when the application or instance changes.
func (r *InstanceConfig) Diff(_ context.Context, req infer.DiffRequest[InstanceConfigArgs, InstanceConfigState]) (infer.DiffResponse, error) {
	diff := map[string]pgp.PropertyDiff{}
	if req.State.ApplicationID != req.Inputs.ApplicationID {
		diff["applicationId"] = pgp.PropertyDiff{Kind: pgp.UpdateReplace}
	}
	if req.State.InstanceID != req.Inputs.InstanceID {
		diff["instanceId"] = pgp.PropertyDiff{Kind: pgp.UpdateReplace}
	}
	oldCfg, oldErr := normalizeConfigJSON(req.State.Config)
	newCfg, newErr := normalizeConfigJSON(req.Inputs.Config)
	if oldErr != nil || newErr != nil || oldCfg != newCfg {
		diff["config"] = pgp.PropertyDiff{Kind: pgp.Update}
	}
	return infer.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}

func (r *InstanceConfig) Create(ctx context.Context, req infer.CreateRequest[InstanceConfigArgs]) (infer.CreateResponse[InstanceConfigState], error) {
	in := req.Inputs
	cfg, err := parseConfigObject(in.Config)
	if err != nil {
		return infer.CreateResponse[InstanceConfigState]{}, err
	}
	norm, err := canonicalConfigJSON(cfg)
	if err != nil {
		return infer.CreateResponse[InstanceConfigState]{}, err
	}
	in.Config = norm
	id := instanceConfigID(in.ApplicationID, in.InstanceID)
	if req.DryRun {
		return infer.CreateResponse[InstanceConfigState]{ID: id, Output: InstanceConfigState{InstanceConfigArgs: in}}, nil
	}
	c, err := platformClient(ctx)
	if err != nil {
		return infer.CreateResponse[InstanceConfigState]{}, err
	}
	version, err := putInstanceConfig(ctx, c, in.ApplicationID, in.InstanceID, cfg)
	if err != nil {
		return infer.CreateResponse[InstanceConfigState]{}, fmt.Errorf("clerk: apply instance config for %s: %w", id, err)
	}
	return infer.CreateResponse[InstanceConfigState]{ID: id, Output: InstanceConfigState{InstanceConfigArgs: in, ConfigVersion: version}}, nil
}

func (r *InstanceConfig) Update(ctx context.Context, req infer.UpdateRequest[InstanceConfigArgs, InstanceConfigState]) (infer.UpdateResponse[InstanceConfigState], error) {
	in := req.Inputs
	cfg, err := parseConfigObject(in.Config)
	if err != nil {
		return infer.UpdateResponse[InstanceConfigState]{}, err
	}
	norm, err := canonicalConfigJSON(cfg)
	if err != nil {
		return infer.UpdateResponse[InstanceConfigState]{}, err
	}
	in.Config = norm
	if req.DryRun {
		return infer.UpdateResponse[InstanceConfigState]{Output: InstanceConfigState{InstanceConfigArgs: in, ConfigVersion: req.State.ConfigVersion}}, nil
	}
	c, err := platformClient(ctx)
	if err != nil {
		return infer.UpdateResponse[InstanceConfigState]{}, err
	}
	version, err := putInstanceConfig(ctx, c, in.ApplicationID, in.InstanceID, cfg)
	if err != nil {
		return infer.UpdateResponse[InstanceConfigState]{}, fmt.Errorf("clerk: update instance config for %s: %w", req.ID, err)
	}
	return infer.UpdateResponse[InstanceConfigState]{Output: InstanceConfigState{InstanceConfigArgs: in, ConfigVersion: version}}, nil
}

func (r *InstanceConfig) Read(ctx context.Context, req infer.ReadRequest[InstanceConfigArgs, InstanceConfigState]) (infer.ReadResponse[InstanceConfigArgs, InstanceConfigState], error) {
	empty := infer.ReadResponse[InstanceConfigArgs, InstanceConfigState]{}
	appID, instID, ok := strings.Cut(req.ID, "/")
	if !ok || appID == "" || instID == "" || strings.Contains(instID, "/") {
		return empty, fmt.Errorf("clerk: invalid instance config ID %q: expected <applicationId>/<instanceId>", req.ID)
	}
	c, err := platformClient(ctx)
	if err != nil {
		return empty, err
	}
	full, version, err := getInstanceConfigRaw(ctx, c, appID, instID)
	if err != nil {
		if isNotFound(err) {
			if err := notFoundOnImport("instance config", req.ID, req.Inputs, req.State); err != nil {
				return empty, err
			}
			return empty, nil
		}
		return empty, fmt.Errorf("clerk: read instance config %q: %w", req.ID, err)
	}
	// Report only the keys Pulumi manages; an import has none, so it adopts
	// the whole configuration.
	shown := full
	managedSrc := req.Inputs.Config
	if managedSrc == "" {
		managedSrc = req.State.Config
	}
	if managedSrc != "" {
		if managed, err := parseConfigObject(managedSrc); err == nil {
			shown = filterConfigToManaged(managed, full)
		}
	}
	cfgJSON, err := canonicalConfigJSON(shown)
	if err != nil {
		return empty, fmt.Errorf("clerk: encode instance config %q: %w", req.ID, err)
	}
	args := InstanceConfigArgs{ApplicationID: appID, InstanceID: instID, Config: cfgJSON}
	return infer.ReadResponse[InstanceConfigArgs, InstanceConfigState]{
		ID: req.ID, Inputs: args, State: InstanceConfigState{InstanceConfigArgs: args, ConfigVersion: version},
	}, nil
}

// Delete only forgets the resource: the Platform API has no way to reset an
// instance configuration, and the Terraform provider did the same.
func (r *InstanceConfig) Delete(_ context.Context, _ infer.DeleteRequest[InstanceConfigState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

// GetInstanceConfig reads the configuration of an instance.
type GetInstanceConfig struct{}

// GetInstanceConfigArgs selects the instance.
type GetInstanceConfigArgs struct {
	ApplicationID string `pulumi:"applicationId"`
	InstanceID    string `pulumi:"instanceId"`
}

// GetInstanceConfigResult is the instance configuration.
type GetInstanceConfigResult struct {
	ID            string `pulumi:"instanceConfigId"`
	Config        string `pulumi:"config" provider:"secret"`
	ConfigVersion string `pulumi:"configVersion"`
}

func (f *GetInstanceConfig) Annotate(a infer.Annotator) {
	a.Describe(&f, "Read the whole configuration of a Clerk instance through the Platform API (beta). The JSON can contain provider client secrets; handle the result accordingly.")
}

func (a *GetInstanceConfigArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ApplicationID, "The ID of the Clerk application.")
	ann.Describe(&a.InstanceID, "The ID of the Clerk instance.")
}

func (r *GetInstanceConfigResult) Annotate(ann infer.Annotator) {
	ann.Describe(&r.ID, "The ID in the form <applicationId>/<instanceId>.")
	ann.Describe(&r.Config, "The instance configuration as a JSON object with sorted keys. It is a secret because instance configuration can contain provider client secrets.")
	ann.Describe(&r.ConfigVersion, "The version of the instance configuration.")
}

func (f *GetInstanceConfig) Invoke(ctx context.Context, req infer.FunctionRequest[GetInstanceConfigArgs]) (infer.FunctionResponse[GetInstanceConfigResult], error) {
	out := infer.FunctionResponse[GetInstanceConfigResult]{}
	c, err := platformClient(ctx)
	if err != nil {
		return out, err
	}
	id := instanceConfigID(req.Input.ApplicationID, req.Input.InstanceID)
	full, version, err := getInstanceConfigRaw(ctx, c, req.Input.ApplicationID, req.Input.InstanceID)
	if err != nil {
		return out, lookupErr("instance config", fmt.Sprintf("%q", id), err)
	}
	cfgJSON, err := canonicalConfigJSON(full)
	if err != nil {
		return out, err
	}
	out.Output = GetInstanceConfigResult{ID: id, Config: cfgJSON, ConfigVersion: version}
	return out, nil
}
