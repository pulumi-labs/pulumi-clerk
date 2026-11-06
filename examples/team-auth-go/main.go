package main

import (
	"encoding/json"

	"github.com/pulumi-labs/pulumi-clerk/sdk/go/clerk"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		orgName := cfg.Get("orgName")
		if orgName == "" {
			orgName = "Acme Engineering"
		}
		orgSlug := cfg.Get("orgSlug")
		if orgSlug == "" {
			orgSlug = "acme-eng"
		}

		// Organization -- top-level B2B tenant.
		org, err := clerk.NewOrganization(ctx, "org", &clerk.OrganizationArgs{
			Name: pulumi.String(orgName),
			Slug: pulumi.String(orgSlug),
		})
		if err != nil {
			return err
		}

		// Custom permission: write access to repositories.
		reposWritePermission, err := clerk.NewOrganizationPermission(ctx, "repos-write", &clerk.OrganizationPermissionArgs{
			Key:         pulumi.String("org:repos:write"),
			Name:        pulumi.String("Write to Repositories"),
			Description: pulumi.String("Permission to push code and merge pull requests"),
		})
		if err != nil {
			return err
		}

		// Senior engineer role -- can merge and deploy.
		seniorEngineerRole, err := clerk.NewOrganizationRole(ctx, "senior-engineer", &clerk.OrganizationRoleArgs{
			Key:         pulumi.String("org:senior-engineer"),
			Name:        pulumi.String("Senior Engineer"),
			Description: pulumi.String("Senior engineers with merge and deploy access"),
			Permissions: pulumi.StringArray{reposWritePermission.ID().ToStringOutput()},
		})
		if err != nil {
			return err
		}

		// Org-aware JWT template -- embeds org context in tokens.
		orgAwareClaims, err := json.Marshal(map[string]string{
			"user_id":  "{{user.id}}",
			"org_id":   "{{org.id}}",
			"org_slug": "{{org.slug}}",
			"org_role": "{{org.role}}",
		})
		if err != nil {
			return err
		}
		orgAwareTemplate, err := clerk.NewJwtTemplate(ctx, "org-aware", &clerk.JwtTemplateArgs{
			Name:     pulumi.String("org_aware"),
			Claims:   pulumi.String(string(orgAwareClaims)),
			Lifetime: pulumi.Int(3600),
		})
		if err != nil {
			return err
		}

		// OAuth application for CI/CD pipeline integration.
		cicdApp, err := clerk.NewOauthApplication(ctx, "cicd", &clerk.OauthApplicationArgs{
			Name:        pulumi.String("CI/CD Integration"),
			CallbackUrl: pulumi.String("https://ci.example.com/oauth/callback"),
			Scopes:      pulumi.String("profile email"),
			Public:      pulumi.Bool(false),
		})
		if err != nil {
			return err
		}

		ctx.Export("orgId", org.ID())
		ctx.Export("seniorEngineerRoleKey", seniorEngineerRole.Key)
		ctx.Export("orgAwareTemplateId", orgAwareTemplate.ID())
		ctx.Export("cicdAppId", cicdApp.ID())
		return nil
	})
}
