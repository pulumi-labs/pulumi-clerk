package main

import (
	"github.com/pulumi/pulumi-clerk/sdk/go/clerk"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		templateNameVal := cfg.Get("templateName")
		if templateNameVal == "" {
			templateNameVal = "access_token"
		}

		claimsJsonVal := cfg.Get("claimsJson")
		if claimsJsonVal == "" {
			claimsJsonVal = "{}"
		}

		// The default provider reads the key from `clerk:apiKey` or CLERK_API_KEY.

		// Create a JWT template to customize claims on tokens issued by this Clerk instance.
		jwtTemplate, err := clerk.NewJwtTemplate(ctx, "jwt-template", &clerk.JwtTemplateArgs{
			Name:   pulumi.String(templateNameVal),
			Claims: pulumi.String(claimsJsonVal),
		})
		if err != nil {
			return err
		}

		// Export the template ID for downstream use.
		ctx.Export("templateId", jwtTemplate.ID())
		return nil
	})
}
