package myproject;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.clerk.JwtTemplate;
import com.pulumi.clerk.JwtTemplateArgs;
import com.pulumi.clerk.OauthApplication;
import com.pulumi.clerk.OauthApplicationArgs;
import com.pulumi.clerk.Organization;
import com.pulumi.clerk.OrganizationArgs;
import com.pulumi.clerk.OrganizationPermission;
import com.pulumi.clerk.OrganizationPermissionArgs;
import com.pulumi.clerk.OrganizationRole;
import com.pulumi.clerk.OrganizationRoleArgs;
import java.util.List;
import java.util.Map;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var config = ctx.config();
        var orgName = config.get("orgName").orElse("Acme Engineering");
        var orgSlug = config.get("orgSlug").orElse("acme-eng");

        // Organization -- top-level B2B tenant.
        var org = new Organization("org", OrganizationArgs.builder()
            .name(orgName)
            .slug(orgSlug)
            .build());

        // Custom permission: write access to repositories.
        var reposWritePermission = new OrganizationPermission("repos-write",
            OrganizationPermissionArgs.builder()
                .key("org:repos:write")
                .name("Write to Repositories")
                .description("Permission to push code and merge pull requests")
                .build());

        // Senior engineer role -- can merge and deploy.
        var seniorEngineerRole = new OrganizationRole("senior-engineer",
            OrganizationRoleArgs.builder()
                .key("org:senior-engineer")
                .name("Senior Engineer")
                .description("Senior engineers with merge and deploy access")
                .permissions(reposWritePermission.key().applyValue(List::of))
                .build());

        // Org-aware JWT template -- embeds org context in tokens.
        String orgAwareClaims;
        try {
            orgAwareClaims = new ObjectMapper().writeValueAsString(Map.of(
                "user_id", "{{user.id}}",
                "org_id", "{{org.id}}",
                "org_slug", "{{org.slug}}",
                "org_role", "{{org.role}}"
            ));
        } catch (Exception e) {
            throw new RuntimeException("Failed to serialize JWT claims", e);
        }
        var orgAwareTemplate = new JwtTemplate("org-aware", JwtTemplateArgs.builder()
            .name("org_aware")
            .claims(orgAwareClaims)
            .lifetime(3600)
            .build());

        // OAuth application for CI/CD pipeline integration.
        var cicdApp = new OauthApplication("cicd", OauthApplicationArgs.builder()
            .name("CI/CD Integration")
            .redirectUris(List.of("https://ci.example.com/oauth/callback"))
            .scopes("profile email")
            .public_(false)
            .build());

        ctx.export("orgId", org.id());
        ctx.export("seniorEngineerRoleKey", seniorEngineerRole.key());
        ctx.export("orgAwareTemplateId", orgAwareTemplate.id());
        ctx.export("cicdAppId", cicdApp.id());
    }
}
