# Clerk Pulumi Provider

A Pulumi provider for [Clerk](https://clerk.com) -- the complete authentication and user
management platform for modern web applications. Provision Clerk resources as infrastructure
code using your favorite language.

This provider bridges the community
[buildwithdeck/terraform-provider-clerk](https://github.com/buildwithdeck/terraform-provider-clerk)
(v0.6.1) via the Pulumi Terraform Bridge (TPF path).

## Resources

| Pulumi Token | Terraform Type | Description |
|---|---|---|
| `clerk:index:AllowlistIdentifier` | `clerk_allowlist_identifier` | Allowlist entries for restricted-signup instances |
| `clerk:index:ApiKey` | `clerk_api_key` | Static API keys for M2M authentication |
| `clerk:index:Application` | `clerk_application` | (Platform API beta) Provision entire Clerk applications |
| `clerk:index:ApplicationSettings` | `clerk_application_settings` | Instance-level organization feature settings |
| `clerk:index:BlocklistIdentifier` | `clerk_blocklist_identifier` | Blocklist entries to prevent specific users |
| `clerk:index:Domain` | `clerk_domain` | (Platform API beta) Application-level custom domains |
| `clerk:index:InstanceConfig` | `clerk_instance_config` | (Platform API beta) Instance-wide auth configuration |
| `clerk:index:InstanceDomain` | `clerk_instance_domain` | (Platform API beta) Per-instance domain assignment |
| `clerk:index:JwtTemplate` | `clerk_jwt_template` | Custom JWT claim templates |
| `clerk:index:Machine` | `clerk_machine` | M2M tokens / service accounts |
| `clerk:index:OauthApplication` | `clerk_oauth_application` | OAuth 2.0 application registrations |
| `clerk:index:Organization` | `clerk_organization` | Organizations for B2B multi-tenancy |
| `clerk:index:OrganizationDomain` | `clerk_organization_domain` | Email-domain-based organization auto-join |
| `clerk:index:OrganizationMembership` | `clerk_organization_membership` | User memberships in organizations |
| `clerk:index:OrganizationPermission` | `clerk_organization_permission` | Custom permissions scoped to organizations |
| `clerk:index:OrganizationRole` | `clerk_organization_role` | Custom org roles with permission lists |
| `clerk:index:RedirectUrl` | `clerk_redirect_url` | OAuth redirect URL allowlist |
| `clerk:index:RoleSet` | `clerk_role_set` | Named collection of OrganizationRoles |
| `clerk:index:SamlConnection` | `clerk_saml_connection` | SAML SSO connection for enterprise auth |
| `clerk:index:SvixWebhook` | `clerk_svix_webhook` | Webhook endpoint subscriptions via Svix |

## Data Sources

| Pulumi Token | Terraform Type | Description |
|---|---|---|
| `clerk:index:getApplication` | `clerk_application` | Look up Clerk application metadata (Platform API beta) |

> **Platform API Note**: Resources marked "(Platform API beta)" require a Platform API key
> (`CLERK_PLATFORM_API_KEY`). These are currently in beta and require a separate API key
> from the standard instance API key. Most organizations only need the standard instance
> API key (`CLERK_API_KEY`) to use this provider.

## Provider Configuration

Configure the provider with your Clerk API credentials:

```typescript
import * as pulumi from "@pulumi/pulumi";

// Set via environment variable CLERK_API_KEY, or via config:
const config = new pulumi.Config();
const apiKey = config.requireSecret("apiKey");
```

### Environment Variables

| Variable | Description |
|---|---|
| `CLERK_API_KEY` | Instance API key (required for most resources) |
| `CLERK_PLATFORM_API_KEY` | Platform API key (required for Application, Domain, InstanceConfig, InstanceDomain) |

## Usage Examples

### TypeScript -- JWT Template

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as clerk from "@pulumi/clerk";

const config = new pulumi.Config();
const apiKey = config.requireSecret("apiKey");

const template = new clerk.JwtTemplate("access-token", {
    name: "access_token",
    claims: JSON.stringify({
        "user_id": "{{user.id}}",
        "email": "{{user.primary_email_address}}",
    }),
});

export const templateId = template.id;
```

### Python -- Organization with Role

```python
import pulumi
import pulumi_clerk as clerk

config = pulumi.Config()
api_key = config.require_secret("apiKey")

org = clerk.Organization("acme",
    name="Acme Engineering",
    slug="acme-eng",
)

permission = clerk.OrganizationPermission("repos-write",
    key="org:repos:write",
    name="Write to Repositories",
    description="Permission to push code and merge pull requests",
)

role = clerk.OrganizationRole("senior-engineer",
    key="org:senior-engineer",
    name="Senior Engineer",
    description="Senior engineers with merge access",
    permissions=[permission.key],
)

pulumi.export("orgId", org.id)
pulumi.export("roleKey", role.key)
```

## Templates and Examples

| Directory | Description |
|---|---|
| `templates/clerk-cloud-ts` | Starter: single JWT template (TypeScript) |
| `templates/clerk-cloud-python` | Starter: single JWT template (Python) |
| `templates/clerk-cloud-go` | Starter: single JWT template (Go) |
| `templates/clerk-cloud-csharp` | Starter: single JWT template (C#) |
| `templates/clerk-cloud-java` | Starter: single JWT template (Java) |
| `templates/clerk-cloud-yaml` | Starter: single JWT template (YAML) |
| `examples/jwt-templates-ts` | Two JWT templates (access + id token) |
| `examples/jwt-templates-python` | Two JWT templates (access + id token) |
| `examples/jwt-templates-go` | Two JWT templates (access + id token) |
| `examples/jwt-templates-csharp` | Two JWT templates (access + id token) |
| `examples/jwt-templates-java` | Two JWT templates (access + id token) |
| `examples/jwt-templates-yaml` | Two JWT templates (access + id token) |
| `examples/team-auth-ts` | Full team auth: org, roles, permissions, JWT, OAuth |
| `examples/team-auth-python` | Full team auth: org, roles, permissions, JWT, OAuth |
| `examples/team-auth-go` | Full team auth: org, roles, permissions, JWT, OAuth |
| `examples/team-auth-csharp` | Full team auth: org, roles, permissions, JWT, OAuth |
| `examples/team-auth-java` | Full team auth: org, roles, permissions, JWT, OAuth |
| `examples/team-auth-yaml` | Full team auth: org, roles, permissions, JWT, OAuth |

## Build from Source

### Prerequisites

- Go 1.25+
- [pulumictl](https://github.com/pulumi/pulumictl) (for version injection)
- pulumi CLI (for schema generation and SDK generation)

### Build the provider binary

```bash
cd lib/providers/clerk
make tidy    # go mod tidy for shim and provider
make build   # builds pulumi-resource-clerk and pulumi-tfgen-clerk to bin/
```

### Run tests

```bash
make test    # go test ./... on the provider module
```

### Generate the schema and SDKs

```bash
make generate_schema   # runs pulumi-tfgen-clerk -> schema.json
make generate_sdks     # runs pulumi package gen-sdk for all 5 languages
```

> **Note**: schema.json and sdk/ directories are gitignored. After generation, install
> the provider locally with `make install_provider` and reference the local SDK with
> `replace` directives in your go.mod (for Go) or local NuGet sources (for .NET).

## Versioning

This provider is pinned to upstream `buildwithdeck/terraform-provider-clerk` **v0.6.1**.
The Clerk TF provider is pre-1.0; schema changes between releases may require re-bridging.
See `skills/SKILLS.md` for guidance when upgrading the upstream version.
