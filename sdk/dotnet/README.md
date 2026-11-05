# Clerk Pulumi Provider

> [!NOTE]
> **Experimental.** This provider is experimental, versions are `0.x`, and its API may change without notice.

A Pulumi provider for [Clerk](https://clerk.com), the authentication and user management platform for modern web
applications. Provision Clerk configuration (allowlists, JWT templates, organizations, roles, SSO and more) as infrastructure
code in your favorite language.

This is a native provider (built on `pulumi-go-provider`) that talks to the Clerk Backend API and the Platform API (beta)
directly. It replaced an earlier provider bridged from `buildwithdeck/terraform-provider-clerk`; see the CHANGELOG for what
changed. [`research/coverage/clerk.md`](../../../research/coverage/clerk.md) lists every Clerk SDK service and API group and what
the provider does with it.

## Resources and lookup functions

Every resource has a `get<Name>` lookup function that reads an existing object (by ID, or by name where Clerk has one).

| Resource | Lookup function | Description |
|---|---|---|
| `clerk:index:AllowlistIdentifier` | `clerk:index:getAllowlistIdentifier` | Allowlist entries for restricted-signup instances |
| `clerk:index:ApiKey` | `clerk:index:getApiKey` | Static API keys for machine-to-machine authentication |
| `clerk:index:Application` | `clerk:index:getApplication` | (Platform API beta) Provision entire Clerk applications |
| `clerk:index:ApplicationSettings` | `clerk:index:getApplicationSettings` | Instance-level organization feature settings |
| `clerk:index:BlocklistIdentifier` | `clerk:index:getBlocklistIdentifier` | Blocklist entries to prevent specific users from signing up |
| `clerk:index:Domain` | `clerk:index:getDomain` | (Platform API beta) Application-level custom domains |
| `clerk:index:InstanceConfig` | `clerk:index:getInstanceConfig` | (Platform API beta) Instance-wide authentication configuration |
| `clerk:index:InstanceDomain` | `clerk:index:getInstanceDomain` | Satellite and primary domains of the instance |
| `clerk:index:JwtTemplate` | `clerk:index:getJwtTemplate` | Custom JWT claim templates |
| `clerk:index:Machine` | `clerk:index:getMachine` | Machine identities (service accounts) with secret keys |
| `clerk:index:OauthApplication` | `clerk:index:getOauthApplication` | OAuth 2.0 application registrations |
| `clerk:index:Organization` | `clerk:index:getOrganization` | Organizations for B2B multi-tenancy |
| `clerk:index:OrganizationDomain` | `clerk:index:getOrganizationDomain` | Email-domain-based organization auto-join rules |
| `clerk:index:OrganizationMembership` | `clerk:index:getOrganizationMembership` | User memberships in organizations |
| `clerk:index:OrganizationPermission` | `clerk:index:getOrganizationPermission` | Custom permissions scoped to organizations |
| `clerk:index:OrganizationRole` | `clerk:index:getOrganizationRole` | Custom organization roles with permission lists |
| `clerk:index:RedirectUrl` | `clerk:index:getRedirectUrl` | OAuth redirect URL allowlist |
| `clerk:index:RoleSet` | `clerk:index:getRoleSet` | Named collection of organization roles |
| `clerk:index:SamlConnection` | `clerk:index:getSamlConnection` | SAML SSO connection for enterprise authentication |
| `clerk:index:SvixWebhook` | `clerk:index:getSvixWebhook` | The Svix application that delivers webhooks |

> **Platform API note**: resources marked "(Platform API beta)" need a Platform API key, which is separate from the instance
> secret key. The Platform API is in beta: request access from Clerk. Everything else needs only the instance secret key.

## Provider Configuration

Resources use the default provider, which reads `clerk:apiKey` from stack config or the environment. No explicit provider
resource is needed:

```bash
pulumi config set --secret clerk:apiKey sk_test_...
# or: export CLERK_API_KEY=sk_test_...   (CLERK_SECRET_KEY works too)
# Platform API (beta): pulumi config set --secret clerk:platformApiKey ak_...  # or CLERK_PLATFORM_API_KEY
```

An explicit `clerk.Provider` is still supported, for example to target several Clerk instances from one program.

| Config | Environment variable | Description |
|---|---|---|
| `clerk:apiKey` (secret) | `CLERK_API_KEY`, then `CLERK_SECRET_KEY` | Instance secret key (`sk_test_...` or `sk_live_...`). Required by every resource except Application, Domain and InstanceConfig. |
| `clerk:platformApiKey` (secret) | `CLERK_PLATFORM_API_KEY` | Platform API key. Required by Application, Domain and InstanceConfig. |
| `clerk:apiUrl` | `CLERK_API_URL` | Override the API base URL (default `https://api.clerk.com/v1`). Only for tests and proxies. |

## Importing

Every resource can be imported by ID. Nested resources use a composite ID:

| Resource | Import ID |
|---|---|
| `OrganizationDomain`, `OrganizationMembership` | `<organizationId>/<id>` (the membership's second half is the user ID) |
| `Domain` | `<applicationId>/<domainId>` |
| `InstanceConfig` | `<applicationId>/<instanceId>` |
| `SvixWebhook` | `svix_webhook` |
| `ApplicationSettings` | `application_settings` |
| anything else | the object's Clerk ID |

Write-only values (an OAuth application's client secret, a JWT template's signing key) cannot be recovered from Clerk when
importing an object that was created elsewhere; Machine and ApiKey secrets can be.

## Usage Examples

### TypeScript -- JWT Template

```typescript
import * as clerk from "@pulumi-labs/clerk";

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
import pulumi_labs_clerk as clerk

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

- Go 1.26+
- pulumi CLI (for schema and SDK generation)

### Build the provider binary

```bash
cd lib/providers/clerk
make build   # builds bin/pulumi-resource-clerk
```

### Run tests

```bash
make test    # go test ./... against an in-memory fake of the Clerk API
```

### Generate the SDKs

```bash
make gen-sdk   # builds, installs the plugin locally, then runs pulumi package gen-sdk for all five languages
```

> **Note**: `schema.json` and `sdk/` are gitignored build output.
