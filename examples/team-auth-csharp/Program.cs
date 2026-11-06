using Pulumi;
using Clerk = Pulumi.Labs.Clerk;
using System.Collections.Generic;
using System.Text.Json;

return await Deployment.RunAsync(() =>
{
    var config = new global::Pulumi.Config();
    var orgName = config.Get("orgName") ?? "Acme Engineering";
    var orgSlug = config.Get("orgSlug") ?? "acme-eng";

    // Organization -- top-level B2B tenant.
    var org = new Clerk.Organization("org", new Clerk.OrganizationArgs
    {
        Name = orgName,
        Slug = orgSlug,
    });

    // Custom permission: write access to repositories.
    var reposWritePermission = new Clerk.OrganizationPermission("repos-write",
        new Clerk.OrganizationPermissionArgs
        {
            Key = "org:repos:write",
            Name = "Write to Repositories",
            Description = "Permission to push code and merge pull requests",
        });

    // Senior engineer role -- can merge and deploy.
    var seniorEngineerRole = new Clerk.OrganizationRole("senior-engineer",
        new Clerk.OrganizationRoleArgs
        {
            Key = "org:senior-engineer",
            Name = "Senior Engineer",
            Description = "Senior engineers with merge and deploy access",
            Permissions = new[] { reposWritePermission.Id },
        });

    // Org-aware JWT template -- embeds org context in tokens.
    var orgAwareClaims = JsonSerializer.Serialize(new
    {
        user_id = "{{user.id}}",
        org_id = "{{org.id}}",
        org_slug = "{{org.slug}}",
        org_role = "{{org.role}}",
    });
    var orgAwareTemplate = new Clerk.JwtTemplate("org-aware", new Clerk.JwtTemplateArgs
    {
        Name = "org_aware",
        Claims = orgAwareClaims,
        Lifetime = 3600,
    });

    // OAuth application for CI/CD pipeline integration.
    var cicdApp = new Clerk.OauthApplication("cicd", new Clerk.OauthApplicationArgs
    {
        Name = "CI/CD Integration",
        CallbackUrl = "https://ci.example.com/oauth/callback",
        Scopes = "profile email",
        Public = false,
    });

    return new Dictionary<string, object?>
    {
        ["orgId"] = org.Id,
        ["seniorEngineerRoleKey"] = seniorEngineerRole.Key,
        ["orgAwareTemplateId"] = orgAwareTemplate.Id,
        ["cicdAppId"] = cicdApp.Id,
    };
});
