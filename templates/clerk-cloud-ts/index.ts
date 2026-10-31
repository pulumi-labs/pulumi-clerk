import * as pulumi from "@pulumi/pulumi";
import * as clerk from "@pulumi/clerk";

const config = new pulumi.Config();
const templateName = config.get("templateName") ?? "access_token";
const claimsJson = config.get("claimsJson") ?? "{}";

// The default provider reads the key from `clerk:apiKey` or CLERK_API_KEY.

// Create a JWT template to customize claims on tokens issued by this Clerk instance.
const jwtTemplate = new clerk.JwtTemplate("jwt-template", {
    name: templateName,
    claims: claimsJson,
});

// Export the template ID for downstream use (e.g. wiring into your application config).
export const templateId = jwtTemplate.id;
