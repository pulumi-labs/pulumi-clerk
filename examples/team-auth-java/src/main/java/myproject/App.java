package myproject;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.labs.clerk.JwtTemplate;
import com.pulumi.labs.clerk.JwtTemplateArgs;
import com.pulumi.labs.clerk.OauthApplication;
import com.pulumi.labs.clerk.OauthApplicationArgs;
import com.pulumi.labs.clerk.Organization;
import com.pulumi.labs.clerk.OrganizationArgs;
import com.pulumi.labs.clerk.OrganizationPermission;
import com.pulumi.labs.clerk.OrganizationPermissionArgs;
import com.pulumi.labs.clerk.OrganizationRole;
import com.pulumi.labs.clerk.OrganizationRoleArgs;
import java.util.List;

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
                .permissions(reposWritePermission.id().applyValue(List::of))
                .build());

        // Org-aware JWT template -- embeds org context in tokens.
        var orgAwareClaims = json(
            "user_id", "{{user.id}}",
            "org_id", "{{org.id}}",
            "org_slug", "{{org.slug}}",
            "org_role", "{{org.role}}");
        var orgAwareTemplate = new JwtTemplate("org-aware", JwtTemplateArgs.builder()
            .name("org_aware")
            .claims(orgAwareClaims)
            .lifetime(3600)
            .build());

        // OAuth application for CI/CD pipeline integration.
        var cicdApp = new OauthApplication("cicd", OauthApplicationArgs.builder()
            .name("CI/CD Integration")
            .callbackUrl("https://ci.example.com/oauth/callback")
            .scopes("profile email")
            .public_(false)
            .build());

        ctx.export("orgId", org.id());
        ctx.export("seniorEngineerRoleKey", seniorEngineerRole.key());
        ctx.export("orgAwareTemplateId", orgAwareTemplate.id());
        ctx.export("cicdAppId", cicdApp.id());
    }

    // Builds a flat JSON object from alternating keys and values (all strings).
    private static String json(String... pairs) {
        var sb = new StringBuilder("{");
        for (int i = 0; i < pairs.length; i += 2) {
            if (i > 0) {
                sb.append(",");
            }
            sb.append('"').append(pairs[i]).append("\":\"").append(pairs[i + 1]).append('"');
        }
        return sb.append("}").toString();
    }
}
