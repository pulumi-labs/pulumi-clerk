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
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"slices"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Application is a Clerk application managed through the Platform API (beta).
type Application struct{}

// ApplicationInstance is one environment (development, production) of an
// application.
type ApplicationInstance struct {
	InstanceID      string `pulumi:"instanceId"`
	EnvironmentType string `pulumi:"environmentType"`
	SecretKey       string `pulumi:"secretKey" provider:"secret"`
	PublishableKey  string `pulumi:"publishableKey"`
}

func (i *ApplicationInstance) Annotate(a infer.Annotator) {
	a.Describe(&i.InstanceID, "Unique identifier of the instance.")
	a.Describe(&i.EnvironmentType, "Environment type, for example development or production.")
	a.Describe(&i.SecretKey, "Secret key (sk_...) of this instance.")
	a.Describe(&i.PublishableKey, "Publishable key (pk_...) of this instance.")
}

// ApplicationArgs are the inputs of an Application.
type ApplicationArgs struct {
	Name             string   `pulumi:"name"`
	Domain           *string  `pulumi:"domain,optional" provider:"replaceOnChanges"`
	ProxyPath        *string  `pulumi:"proxyPath,optional" provider:"replaceOnChanges"`
	EnvironmentTypes []string `pulumi:"environmentTypes,optional" provider:"replaceOnChanges"`
	Template         *string  `pulumi:"template,optional" provider:"replaceOnChanges"`
	Logo             *string  `pulumi:"logo,optional" provider:"secret"`
	Favicon          *string  `pulumi:"favicon,optional" provider:"secret"`
}

// ApplicationState is the stored state of an Application.
type ApplicationState struct {
	ApplicationArgs
	Instances []ApplicationInstance `pulumi:"instances" provider:"secret"`
}

func (r *Application) Annotate(a infer.Annotator) {
	a.Describe(&r, "A Clerk application, managed through the Clerk Platform API. The Platform API is in beta and must be enabled by Clerk for your account; configure it with the clerk:platformApiKey provider setting. Only the name, logo and favicon can be updated in place; every other input forces replacement. The API does not return domain, proxyPath, environmentTypes, template, logo or favicon on reads, so those are kept as configured. Import with the application ID (for example: pulumi import clerk:index/application:Application app app_123); an imported application has an empty name until it is set in the program.")
}

func (a *ApplicationArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.Name, "Name of the application.")
	ann.Describe(&a.Domain, "Custom domain for the application. Create-only: changing this forces a new resource.")
	ann.Describe(&a.ProxyPath, "Proxy path for the application. Create-only: changing this forces a new resource.")
	ann.Describe(&a.EnvironmentTypes, "Environment types to create, for example [\"development\", \"production\"]. Create-only: changing this forces a new resource.")
	ann.Describe(&a.Template, "Application template slug. Create-only: changing this forces a new resource.")
	ann.Describe(&a.Logo, "Base64-encoded logo image content (use the language's file-to-base64 helper). Write-only: the API does not return it. Removing it deletes the logo.")
	ann.Describe(&a.Favicon, "Base64-encoded favicon image content. Write-only: the API does not return it. Removing it deletes the favicon.")
}

func (s *ApplicationState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.Instances, "The instances (environments) of the application, with their keys. The whole list is secret because it carries each instance secret key.")
}

var _ infer.CustomResource[ApplicationArgs, ApplicationState] = (*Application)(nil)
var _ infer.CustomRead[ApplicationArgs, ApplicationState] = (*Application)(nil)
var _ infer.CustomUpdate[ApplicationArgs, ApplicationState] = (*Application)(nil)
var _ infer.CustomDelete[ApplicationState] = (*Application)(nil)
var _ infer.CustomDiff[ApplicationArgs, ApplicationState] = (*Application)(nil)
var _ infer.ExplicitDependencies[ApplicationArgs, ApplicationState] = (*Application)(nil)

// Diff replaces on any create-only input and updates in place otherwise. It is
// written out because infer's replaceOnChanges does not cover the elements of
// the environmentTypes list.
func (r *Application) Diff(_ context.Context, req infer.DiffRequest[ApplicationArgs, ApplicationState]) (infer.DiffResponse, error) {
	diff := map[string]pgp.PropertyDiff{}
	kind := func(replace, hadOld, hasNew bool) pgp.DiffKind {
		switch {
		case !hadOld && hasNew:
			if replace {
				return pgp.AddReplace
			}
			return pgp.Add
		case hadOld && !hasNew:
			if replace {
				return pgp.DeleteReplace
			}
			return pgp.Delete
		}
		if replace {
			return pgp.UpdateReplace
		}
		return pgp.Update
	}
	str := func(key string, old, cur *string, replace bool) {
		if derefStr(old) != derefStr(cur) || (old == nil) != (cur == nil) {
			diff[key] = pgp.PropertyDiff{Kind: kind(replace, old != nil, cur != nil), InputDiff: true}
		}
	}
	in, st := req.Inputs, req.State
	if in.Name != st.Name {
		diff["name"] = pgp.PropertyDiff{Kind: pgp.Update, InputDiff: true}
	}
	str("domain", st.Domain, in.Domain, true)
	str("proxyPath", st.ProxyPath, in.ProxyPath, true)
	str("template", st.Template, in.Template, true)
	str("logo", st.Logo, in.Logo, false)
	str("favicon", st.Favicon, in.Favicon, false)
	if !slices.Equal(in.EnvironmentTypes, st.EnvironmentTypes) {
		diff["environmentTypes"] = pgp.PropertyDiff{Kind: kind(true, len(st.EnvironmentTypes) > 0, len(in.EnvironmentTypes) > 0), InputDiff: true}
	}
	return infer.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}

// WireDependencies marks the instance list (which carries every secret key)
// secret on every output path, not only when an input was secret.
func (r *Application) WireDependencies(f infer.FieldSelector, args *ApplicationArgs, state *ApplicationState) {
	f.OutputField(&state.Instances).AlwaysSecret()
}

// applicationJSON is the Platform API shape. The documented response carries
// only the ID and instances; name is read when the API includes it.
type applicationJSON struct {
	ApplicationID string `json:"application_id"`
	Name          string `json:"name"`
	Instances     []struct {
		InstanceID      string `json:"instance_id"`
		EnvironmentType string `json:"environment_type"`
		SecretKey       string `json:"secret_key"`
		PublishableKey  string `json:"publishable_key"`
	} `json:"instances"`
}

// instances converts the API instances, keeping secret keys already known when
// the response omits them.
func (j applicationJSON) instances(prev []ApplicationInstance) []ApplicationInstance {
	out := make([]ApplicationInstance, 0, len(j.Instances))
	for _, in := range j.Instances {
		inst := ApplicationInstance{InstanceID: in.InstanceID, EnvironmentType: in.EnvironmentType, SecretKey: in.SecretKey, PublishableKey: in.PublishableKey}
		for _, p := range prev {
			if p.InstanceID != in.InstanceID {
				continue
			}
			if inst.SecretKey == "" {
				inst.SecretKey = p.SecretKey
			}
			if inst.PublishableKey == "" {
				inst.PublishableKey = p.PublishableKey
			}
		}
		out = append(out, inst)
	}
	return out
}

func getApplicationJSON(ctx context.Context, c *client, id string) (*applicationJSON, error) {
	var out applicationJSON
	if err := c.do(ctx, http.MethodGet, "/platform/applications/"+id+"?include_secret_keys=true", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// uploadImage sets, replaces or (when value is nil) removes an application
// image. kind is "logo" or "favicon".
func uploadImage(ctx context.Context, c *client, appID, kind string, value *string) error {
	path := "/platform/applications/" + appID + "/" + kind
	if value == nil {
		if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil && !isNotFound(err) {
			return fmt.Errorf("clerk: delete application %s: %w", kind, err)
		}
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(*value)
	if err != nil {
		return fmt.Errorf("clerk: application %s is not valid base64: %w", kind, err)
	}
	if err := c.doMultipart(ctx, http.MethodPost, path, "file", data, nil); err != nil {
		return fmt.Errorf("clerk: upload application %s: %w", kind, err)
	}
	return nil
}

func validateImage(kind string, v *string) error {
	if v == nil {
		return nil
	}
	if _, err := base64.StdEncoding.DecodeString(*v); err != nil {
		return fmt.Errorf("clerk: application %s is not valid base64: %w", kind, err)
	}
	return nil
}

func (r *Application) Create(ctx context.Context, req infer.CreateRequest[ApplicationArgs]) (infer.CreateResponse[ApplicationState], error) {
	if err := validateImage("logo", req.Inputs.Logo); err != nil {
		return infer.CreateResponse[ApplicationState]{}, err
	}
	if err := validateImage("favicon", req.Inputs.Favicon); err != nil {
		return infer.CreateResponse[ApplicationState]{}, err
	}
	if req.DryRun {
		return infer.CreateResponse[ApplicationState]{Output: ApplicationState{ApplicationArgs: req.Inputs}}, nil
	}
	c, err := platformClient(ctx)
	if err != nil {
		return infer.CreateResponse[ApplicationState]{}, err
	}
	body := map[string]any{"name": req.Inputs.Name}
	if v := derefStr(req.Inputs.Domain); v != "" {
		body["domain"] = v
	}
	if v := derefStr(req.Inputs.ProxyPath); v != "" {
		body["proxy_path"] = v
	}
	if v := derefStr(req.Inputs.Template); v != "" {
		body["template"] = v
	}
	if len(req.Inputs.EnvironmentTypes) > 0 {
		body["environment_types"] = req.Inputs.EnvironmentTypes
	}
	var out applicationJSON
	if err := c.do(ctx, http.MethodPost, "/platform/applications", body, &out); err != nil {
		return infer.CreateResponse[ApplicationState]{}, fmt.Errorf("clerk: create application %q: %w", req.Inputs.Name, err)
	}
	state := ApplicationState{ApplicationArgs: req.Inputs, Instances: out.instances(nil)}
	resp := infer.CreateResponse[ApplicationState]{ID: out.ApplicationID, Output: state}
	// The application exists now; a failed image upload must not leak it.
	for _, img := range []struct {
		kind string
		val  *string
	}{{"logo", req.Inputs.Logo}, {"favicon", req.Inputs.Favicon}} {
		if img.val == nil {
			continue
		}
		if err := uploadImage(ctx, c, out.ApplicationID, img.kind, img.val); err != nil {
			return resp, infer.ResourceInitFailedError{Reasons: []string{err.Error()}}
		}
	}
	return resp, nil
}

func (r *Application) Read(ctx context.Context, req infer.ReadRequest[ApplicationArgs, ApplicationState]) (infer.ReadResponse[ApplicationArgs, ApplicationState], error) {
	empty := infer.ReadResponse[ApplicationArgs, ApplicationState]{}
	c, err := platformClient(ctx)
	if err != nil {
		return empty, err
	}
	got, err := getApplicationJSON(ctx, c, req.ID)
	if isNotFound(err) {
		return empty, nil
	}
	if err != nil {
		return empty, fmt.Errorf("clerk: read application %q: %w", req.ID, err)
	}
	// Everything but the instances is write-only; keep what Pulumi already has.
	args := req.Inputs
	if args.Name == "" {
		args = req.State.ApplicationArgs
	}
	if got.Name != "" {
		args.Name = got.Name
	}
	state := ApplicationState{ApplicationArgs: args, Instances: got.instances(req.State.Instances)}
	return infer.ReadResponse[ApplicationArgs, ApplicationState]{ID: req.ID, Inputs: args, State: state}, nil
}

func (r *Application) Update(ctx context.Context, req infer.UpdateRequest[ApplicationArgs, ApplicationState]) (infer.UpdateResponse[ApplicationState], error) {
	if err := validateImage("logo", req.Inputs.Logo); err != nil {
		return infer.UpdateResponse[ApplicationState]{}, err
	}
	if err := validateImage("favicon", req.Inputs.Favicon); err != nil {
		return infer.UpdateResponse[ApplicationState]{}, err
	}
	state := ApplicationState{ApplicationArgs: req.Inputs, Instances: req.State.Instances}
	if req.DryRun {
		return infer.UpdateResponse[ApplicationState]{Output: state}, nil
	}
	c, err := platformClient(ctx)
	if err != nil {
		return infer.UpdateResponse[ApplicationState]{}, err
	}
	if req.Inputs.Name != req.State.Name {
		var out applicationJSON
		if err := c.do(ctx, http.MethodPatch, "/platform/applications/"+req.ID, map[string]any{"name": req.Inputs.Name}, &out); err != nil {
			return infer.UpdateResponse[ApplicationState]{}, fmt.Errorf("clerk: update application %q: %w", req.ID, err)
		}
		if len(out.Instances) > 0 {
			state.Instances = out.instances(req.State.Instances)
		}
	}
	for _, img := range []struct {
		kind     string
		old, new *string
	}{{"logo", req.State.Logo, req.Inputs.Logo}, {"favicon", req.State.Favicon, req.Inputs.Favicon}} {
		if derefStr(img.old) == derefStr(img.new) && (img.old == nil) == (img.new == nil) {
			continue
		}
		if err := uploadImage(ctx, c, req.ID, img.kind, img.new); err != nil {
			// Keep the old image in state so the next update retries.
			if img.kind == "logo" {
				state.Logo = img.old
			} else {
				state.Favicon = img.old
			}
			return infer.UpdateResponse[ApplicationState]{Output: state}, infer.ResourceInitFailedError{Reasons: []string{err.Error()}}
		}
	}
	return infer.UpdateResponse[ApplicationState]{Output: state}, nil
}

func (r *Application) Delete(ctx context.Context, req infer.DeleteRequest[ApplicationState]) (infer.DeleteResponse, error) {
	c, err := platformClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := c.do(ctx, http.MethodDelete, "/platform/applications/"+req.ID, nil, nil); err != nil && !isNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("clerk: delete application %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// GetApplication looks up a Clerk application and its instance keys by ID.
type GetApplication struct{}

// GetApplicationArgs selects the application.
type GetApplicationArgs struct {
	ID string `pulumi:"applicationId"`
}

// GetApplicationResult is the looked-up application.
type GetApplicationResult struct {
	ID        string                `pulumi:"applicationId"`
	Instances []ApplicationInstance `pulumi:"instances" provider:"secret"`
}

func (f *GetApplication) Annotate(a infer.Annotator) {
	a.Describe(&f, "Look up a Clerk application by ID through the Platform API (beta), including its instances and keys.")
}

func (a *GetApplicationArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ID, "The application ID to look up.")
}

func (r *GetApplicationResult) Annotate(ann infer.Annotator) {
	ann.Describe(&r.ID, "The application ID.")
	ann.Describe(&r.Instances, "The instances (environments) of the application, with their keys.")
}

func (f *GetApplication) Invoke(ctx context.Context, req infer.FunctionRequest[GetApplicationArgs]) (infer.FunctionResponse[GetApplicationResult], error) {
	out := infer.FunctionResponse[GetApplicationResult]{}
	if req.Input.ID == "" {
		return out, fmt.Errorf("clerk: getApplication needs an id")
	}
	c, err := platformClient(ctx)
	if err != nil {
		return out, err
	}
	got, err := getApplicationJSON(ctx, c, req.Input.ID)
	if err != nil {
		return out, lookupErr("application", fmt.Sprintf("%q", req.Input.ID), err)
	}
	out.Output = GetApplicationResult{ID: got.ApplicationID, Instances: got.instances(nil)}
	if out.Output.ID == "" {
		out.Output.ID = req.Input.ID
	}
	return out, nil
}
