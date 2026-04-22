import * as pulumi from "@pulumi/pulumi";
import * as clerk from "@pulumi/clerk";

const config = new pulumi.Config();
const apiKey = config.requireSecret("apiKey");

// Explicit provider so credentials come from project-namespace config,
// not environment variables.
const provider = new clerk.Provider("clerk", { apiKey });

// Access token template -- adds user identity claims to short-lived access tokens.
const accessTokenTemplate = new clerk.JwtTemplate("access-token", {
    name: "access_token",
    claims: JSON.stringify({
        "user_id": "{{user.id}}",
        "email": "{{user.primary_email_address}}",
        "full_name": "{{user.full_name}}",
    }),
    lifetime: 3600,
    allowedClockSkew: 5,
}, { provider });

// ID token template -- adds richer profile claims for display purposes.
const idTokenTemplate = new clerk.JwtTemplate("id-token", {
    name: "id_token",
    claims: JSON.stringify({
        "sub": "{{user.id}}",
        "email": "{{user.primary_email_address}}",
        "given_name": "{{user.first_name}}",
        "family_name": "{{user.last_name}}",
        "picture": "{{user.image_url}}",
    }),
    lifetime: 3600,
}, { provider });

export const accessTokenTemplateId = accessTokenTemplate.id;
export const idTokenTemplateId = idTokenTemplate.id;
