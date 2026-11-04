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

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// ProviderConfig holds the provider-level configuration.
type ProviderConfig struct {
	// APIKey is the Clerk instance secret key (sk_test_... or sk_live_...)
	// used for the Backend API.
	APIKey *string `pulumi:"apiKey,optional" provider:"secret"`
	// PlatformAPIKey is the Clerk Platform API key (beta) used for
	// applications, domains, instance domains and instance configuration.
	PlatformAPIKey *string `pulumi:"platformApiKey,optional" provider:"secret"`
	// APIURL overrides the Clerk API base URL, for tests and proxies.
	APIURL *string `pulumi:"apiUrl,optional"`
}

// Annotate describes the config fields and wires the environment variable
// fallbacks; infer reads env defaults only from SetDefault.
func (c *ProviderConfig) Annotate(a infer.Annotator) {
	a.SetDefault(&c.APIKey, nil, "CLERK_API_KEY", "CLERK_SECRET_KEY")
	a.SetDefault(&c.PlatformAPIKey, nil, "CLERK_PLATFORM_API_KEY")
	a.SetDefault(&c.APIURL, nil, "CLERK_API_URL")
	a.Describe(&c.APIKey, "Clerk instance secret key (starts with sk_test_ or sk_live_). Required by every resource except Application, Domain, InstanceDomain and InstanceConfig. Falls back to the CLERK_API_KEY environment variable, then CLERK_SECRET_KEY.")
	a.Describe(&c.PlatformAPIKey, "Clerk Platform API key (beta). Required by Application, Domain, InstanceDomain and InstanceConfig. Falls back to the CLERK_PLATFORM_API_KEY environment variable.")
	a.Describe(&c.APIURL, "Override for the Clerk API base URL (default https://api.clerk.com/v1). Falls back to CLERK_API_URL. Only needed for tests and proxies.")
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strPtr(s string) *string { return &s }

// backendClient returns a client for the Backend API, or a clear error when
// no instance secret key is configured.
func backendClient(ctx context.Context) (*client, error) {
	cfg := infer.GetConfig[ProviderConfig](ctx)
	key := derefStr(cfg.APIKey)
	if key == "" {
		return nil, fmt.Errorf("clerk: no instance secret key: set the clerk:apiKey config (pulumi config set --secret clerk:apiKey sk_...) or the CLERK_API_KEY environment variable")
	}
	return newClient(derefStr(cfg.APIURL), key), nil
}

// platformClient returns a client for the Platform API (beta).
func platformClient(ctx context.Context) (*client, error) {
	cfg := infer.GetConfig[ProviderConfig](ctx)
	key := derefStr(cfg.PlatformAPIKey)
	if key == "" {
		return nil, fmt.Errorf("clerk: no Platform API key: set the clerk:platformApiKey config or the CLERK_PLATFORM_API_KEY environment variable (the Clerk Platform API is in beta; request access from Clerk)")
	}
	return newClient(derefStr(cfg.APIURL), key), nil
}

// lookupErr words a failed lookup.
func lookupErr(kind, what string, err error) error {
	if isNotFound(err) {
		return fmt.Errorf("clerk: %s %s was not found", kind, what)
	}
	return fmt.Errorf("clerk: look up %s %s: %w", kind, what, err)
}

// New returns the configured Clerk Pulumi provider.
func New() (pgp.Provider, error) {
	const desc = "Manage Clerk (clerk.com) authentication infrastructure as code: allowlists, blocklists, JWT templates, organizations and their roles and permissions, OAuth applications, SAML connections, API keys, machines and webhooks."
	return infer.NewProviderBuilder().
		WithDisplayName("Clerk").
		WithDescription(desc).
		WithHomepage("https://clerk.com").
		WithRepository("https://github.com/pulumi/autoregistry").
		WithPublisher("Pulumi").
		WithLanguageMap(map[string]any{
			"go": map[string]any{"generateResourceContainerTypes": true, "respectSchemaVersion": true},
			"nodejs": map[string]any{
				"packageName":          "@pulumi/clerk",
				"packageDescription":   desc,
				"respectSchemaVersion": true,
			},
			"python": map[string]any{
				"packageName":          "pulumi_clerk",
				"respectSchemaVersion": true,
				"pyproject":            map[string]any{"enabled": true},
			},
			"csharp": map[string]any{"rootNamespace": "Pulumi", "respectSchemaVersion": true},
			"java":   map[string]any{"basePackage": "com.pulumi", "respectSchemaVersion": true},
		}).
		WithGoImportPath("github.com/pulumi/pulumi-clerk/sdk/go/clerk").
		WithPluginDownloadURL("github://api.github.com/pulumi/autoregistry").
		WithLicense("Apache-2.0").
		WithKeywords("pulumi", "clerk", "auth", "authentication", "identity", "sso", "organizations", "category/infrastructure").
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		}).
		WithConfig(infer.Config(ProviderConfig{})).
		WithResources(
			infer.Resource[*BlocklistIdentifier, BlocklistIdentifierArgs, BlocklistIdentifierState](&BlocklistIdentifier{}),
			infer.Resource[*AllowlistIdentifier, AllowlistIdentifierArgs, AllowlistIdentifierState](&AllowlistIdentifier{}),
			infer.Resource[*RedirectUrl, RedirectUrlArgs, RedirectUrlState](&RedirectUrl{}),
			infer.Resource[*JwtTemplate, JwtTemplateArgs, JwtTemplateState](&JwtTemplate{}),
			infer.Resource[*Machine, MachineArgs, MachineState](&Machine{}),
			infer.Resource[*OauthApplication, OauthApplicationArgs, OauthApplicationState](&OauthApplication{}),
			infer.Resource[*ApiKey, ApiKeyArgs, ApiKeyState](&ApiKey{}),
			infer.Resource[*Organization, OrganizationArgs, OrganizationState](&Organization{}),
			infer.Resource[*OrganizationDomain, OrganizationDomainArgs, OrganizationDomainState](&OrganizationDomain{}),
			infer.Resource[*OrganizationMembership, OrganizationMembershipArgs, OrganizationMembershipState](&OrganizationMembership{}),
			infer.Resource[*OrganizationPermission, OrganizationPermissionArgs, OrganizationPermissionState](&OrganizationPermission{}),
			infer.Resource[*OrganizationRole, OrganizationRoleArgs, OrganizationRoleState](&OrganizationRole{}),
			infer.Resource[*RoleSet, RoleSetArgs, RoleSetState](&RoleSet{}),
			infer.Resource[*SamlConnection, SamlConnectionArgs, SamlConnectionState](&SamlConnection{}),
			infer.Resource[*SvixWebhook, SvixWebhookArgs, SvixWebhookState](&SvixWebhook{}),
			infer.Resource[*InstanceConfig, InstanceConfigArgs, InstanceConfigState](&InstanceConfig{}),
			infer.Resource[*InstanceDomain, InstanceDomainArgs, InstanceDomainState](&InstanceDomain{}),
			infer.Resource[*Application, ApplicationArgs, ApplicationState](&Application{}),
			infer.Resource[*Domain, DomainArgs, DomainState](&Domain{}),
			infer.Resource[*ApplicationSettings, ApplicationSettingsArgs, ApplicationSettingsState](&ApplicationSettings{}),
		).
		WithFunctions(
			infer.Function[*GetBlocklistIdentifier, GetBlocklistIdentifierArgs, GetBlocklistIdentifierResult](&GetBlocklistIdentifier{}),
			infer.Function[*GetAllowlistIdentifier, GetAllowlistIdentifierArgs, GetAllowlistIdentifierResult](&GetAllowlistIdentifier{}),
			infer.Function[*GetRedirectUrl, GetRedirectUrlArgs, GetRedirectUrlResult](&GetRedirectUrl{}),
			infer.Function[*GetJwtTemplate, GetJwtTemplateArgs, GetJwtTemplateResult](&GetJwtTemplate{}),
			infer.Function[*GetMachine, GetMachineArgs, GetMachineResult](&GetMachine{}),
			infer.Function[*GetOauthApplication, GetOauthApplicationArgs, GetOauthApplicationResult](&GetOauthApplication{}),
			infer.Function[*GetApiKey, GetApiKeyArgs, GetApiKeyResult](&GetApiKey{}),
			infer.Function[*GetOrganization, GetOrganizationArgs, GetOrganizationResult](&GetOrganization{}),
			infer.Function[*GetOrganizationDomain, GetOrganizationDomainArgs, GetOrganizationDomainResult](&GetOrganizationDomain{}),
			infer.Function[*GetOrganizationMembership, GetOrganizationMembershipArgs, GetOrganizationMembershipResult](&GetOrganizationMembership{}),
			infer.Function[*GetOrganizationPermission, GetOrganizationPermissionArgs, GetOrganizationPermissionResult](&GetOrganizationPermission{}),
			infer.Function[*GetOrganizationRole, GetOrganizationRoleArgs, GetOrganizationRoleResult](&GetOrganizationRole{}),
			infer.Function[*GetRoleSet, GetRoleSetArgs, GetRoleSetResult](&GetRoleSet{}),
			infer.Function[*GetSamlConnection, GetSamlConnectionArgs, GetSamlConnectionResult](&GetSamlConnection{}),
			infer.Function[*GetSvixWebhook, GetSvixWebhookArgs, GetSvixWebhookResult](&GetSvixWebhook{}),
			infer.Function[*GetInstanceConfig, GetInstanceConfigArgs, GetInstanceConfigResult](&GetInstanceConfig{}),
			infer.Function[*GetInstanceDomain, GetInstanceDomainArgs, GetInstanceDomainResult](&GetInstanceDomain{}),
			infer.Function[*GetApplication, GetApplicationArgs, GetApplicationResult](&GetApplication{}),
			infer.Function[*GetDomain, GetDomainArgs, GetDomainResult](&GetDomain{}),
			infer.Function[*GetApplicationSettings, GetApplicationSettingsArgs, GetApplicationSettingsResult](&GetApplicationSettings{}),
		).
		Build()
}
