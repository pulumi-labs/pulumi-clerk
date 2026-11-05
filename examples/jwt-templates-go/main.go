package main

import (
	"encoding/json"

	"github.com/pulumi-labs/pulumi-clerk/sdk/go/clerk"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Access token template -- adds user identity claims.
		accessClaims, err := json.Marshal(map[string]string{
			"user_id":   "{{user.id}}",
			"email":     "{{user.primary_email_address}}",
			"full_name": "{{user.full_name}}",
		})
		if err != nil {
			return err
		}

		accessTokenTemplate, err := clerk.NewJwtTemplate(ctx, "access-token", &clerk.JwtTemplateArgs{
			Name:             pulumi.String("access_token"),
			Claims:           pulumi.String(string(accessClaims)),
			Lifetime:         pulumi.Int(3600),
			AllowedClockSkew: pulumi.Int(5),
		})
		if err != nil {
			return err
		}

		// ID token template -- adds richer profile claims.
		idClaims, err := json.Marshal(map[string]string{
			"sub":         "{{user.id}}",
			"email":       "{{user.primary_email_address}}",
			"given_name":  "{{user.first_name}}",
			"family_name": "{{user.last_name}}",
			"picture":     "{{user.image_url}}",
		})
		if err != nil {
			return err
		}

		idTokenTemplate, err := clerk.NewJwtTemplate(ctx, "id-token", &clerk.JwtTemplateArgs{
			Name:     pulumi.String("id_token"),
			Claims:   pulumi.String(string(idClaims)),
			Lifetime: pulumi.Int(3600),
		})
		if err != nil {
			return err
		}

		ctx.Export("accessTokenTemplateId", accessTokenTemplate.ID())
		ctx.Export("idTokenTemplateId", idTokenTemplate.ID())
		return nil
	})
}
