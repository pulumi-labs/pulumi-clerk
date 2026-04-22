import json
import pulumi
import pulumi_clerk as clerk

config = pulumi.Config()
api_key = config.require_secret("apiKey")
org_name = config.get("orgName") or "Acme Engineering"
org_slug = config.get("orgSlug") or "acme-eng"

# Explicit provider so credentials come from project-namespace config,
# not environment variables.
provider = clerk.Provider("clerk", api_key=api_key)

# Organization -- top-level B2B tenant.
org = clerk.Organization("org",
    name=org_name,
    slug=org_slug,
    opts=pulumi.ResourceOptions(provider=provider),
)

# Custom permission: write access to repositories.
repos_write_permission = clerk.OrganizationPermission("repos-write",
    key="org:repos:write",
    name="Write to Repositories",
    description="Permission to push code and merge pull requests",
    opts=pulumi.ResourceOptions(provider=provider),
)

# Senior engineer role -- can merge and deploy.
senior_engineer_role = clerk.OrganizationRole("senior-engineer",
    key="org:senior-engineer",
    name="Senior Engineer",
    description="Senior engineers with merge and deploy access",
    permissions=[repos_write_permission.key],
    opts=pulumi.ResourceOptions(provider=provider),
)

# Org-aware JWT template -- embeds org context in tokens.
org_aware_template = clerk.JwtTemplate("org-aware",
    name="org_aware",
    claims=json.dumps({
        "user_id": "{{user.id}}",
        "org_id": "{{org.id}}",
        "org_slug": "{{org.slug}}",
        "org_role": "{{org.role}}",
    }),
    lifetime=3600,
    opts=pulumi.ResourceOptions(provider=provider),
)

# OAuth application for CI/CD pipeline integration.
cicd_app = clerk.OauthApplication("cicd",
    name="CI/CD Integration",
    redirect_uris=["https://ci.example.com/oauth/callback"],
    scopes="profile email",
    public=False,
    opts=pulumi.ResourceOptions(provider=provider),
)

pulumi.export("orgId", org.id)
pulumi.export("seniorEngineerRoleKey", senior_engineer_role.key)
pulumi.export("orgAwareTemplateId", org_aware_template.id)
pulumi.export("cicdAppId", cicd_app.id)
