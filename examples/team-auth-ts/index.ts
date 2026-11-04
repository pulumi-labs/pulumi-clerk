import * as pulumi from "@pulumi/pulumi";
import * as clerk from "@pulumi/clerk";

const config = new pulumi.Config();
const orgName = config.get("orgName") ?? "Acme Engineering";
const orgSlug = config.get("orgSlug") ?? "acme-eng";

// Organization -- top-level B2B tenant.
const org = new clerk.Organization("org", {
    name: orgName,
    slug: orgSlug,
});

// Custom permission: write access to repositories.
const reposWritePermission = new clerk.OrganizationPermission("repos-write", {
    key: "org:repos:write",
    name: "Write to Repositories",
    description: "Permission to push code and merge pull requests",
});

// Senior engineer role -- can merge and deploy.
const seniorEngineerRole = new clerk.OrganizationRole("senior-engineer", {
    key: "org:senior-engineer",
    name: "Senior Engineer",
    description: "Senior engineers with merge and deploy access",
    permissions: [reposWritePermission.key],
});

// Org-aware JWT template -- embeds org context in tokens.
const orgAwareTemplate = new clerk.JwtTemplate("org-aware", {
    name: "org_aware",
    claims: JSON.stringify({
        "user_id": "{{user.id}}",
        "org_id": "{{org.id}}",
        "org_slug": "{{org.slug}}",
        "org_role": "{{org.role}}",
    }),
    lifetime: 3600,
});

// OAuth application for CI/CD pipeline integration.
const cicdApp = new clerk.OauthApplication("cicd", {
    name: "CI/CD Integration",
    callbackUrl: "https://ci.example.com/oauth/callback",
    scopes: "profile email",
    public: false,
});

export const orgId = org.id;
export const seniorEngineerRoleKey = seniorEngineerRole.key;
export const orgAwareTemplateId = orgAwareTemplate.id;
export const cicdAppId = cicdApp.id;
