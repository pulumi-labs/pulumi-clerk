# Changelog

All notable changes to the Clerk Pulumi provider are documented here.

## [Unreleased]

### Added

Provider resources (all bridged from buildwithdeck/terraform-provider-clerk v0.6.1):

- `clerk:index:AllowlistIdentifier` -- Allowlist entries for restricted-signup instances.
- `clerk:index:ApiKey` -- (Instance) Manage static API keys for M2M authentication.
- `clerk:index:Application` -- (Platform API, beta) Provision entire Clerk applications.
- `clerk:index:ApplicationSettings` -- Instance-level organization feature settings.
- `clerk:index:BlocklistIdentifier` -- Blocklist entries to prevent specific users from signing up.
- `clerk:index:Domain` -- (Platform API, beta) Application-level custom domains.
- `clerk:index:InstanceConfig` -- (Platform API, beta) Instance-wide authentication configuration.
- `clerk:index:InstanceDomain` -- (Platform API, beta) Per-instance domain assignment.
- `clerk:index:JwtTemplate` -- Custom JWT claim templates for access and ID tokens.
- `clerk:index:Machine` -- M2M tokens and service accounts.
- `clerk:index:OauthApplication` -- OAuth 2.0 application registrations.
- `clerk:index:Organization` -- Organizations for B2B multi-tenancy.
- `clerk:index:OrganizationDomain` -- Email-domain-based organization auto-join rules.
- `clerk:index:OrganizationMembership` -- User memberships in organizations.
- `clerk:index:OrganizationPermission` -- Custom permissions scoped to organizations.
- `clerk:index:OrganizationRole` -- Custom org roles with associated permission lists.
- `clerk:index:RedirectUrl` -- OAuth redirect URL allowlist entries.
- `clerk:index:RoleSet` -- Named collections of OrganizationRoles.
- `clerk:index:SamlConnection` -- SAML SSO connections for enterprise authentication.
- `clerk:index:SvixWebhook` -- Webhook endpoint subscriptions via the Svix platform.

Provider data sources:

- `clerk:index:getApplication` -- Data source: look up Clerk application metadata (Platform API beta).

Templates (starter scaffolds, one resource each):

- `templates/clerk-cloud-ts` -- TypeScript
- `templates/clerk-cloud-python` -- Python
- `templates/clerk-cloud-go` -- Go
- `templates/clerk-cloud-csharp` -- C#
- `templates/clerk-cloud-java` -- Java
- `templates/clerk-cloud-yaml` -- YAML

Examples (two scenarios, six languages each = 12 examples):

- `examples/jwt-templates-{ts,python,go,csharp,java,yaml}` -- Two JWT templates
  demonstrating access_token and id_token claim patterns.
- `examples/team-auth-{ts,python,go,csharp,java,yaml}` -- Team authorization scenario:
  Organization, OrganizationRole, OrganizationPermission, JwtTemplate (org-aware claims),
  and OauthApplication (CI/CD integration). Uses instance API key only.
