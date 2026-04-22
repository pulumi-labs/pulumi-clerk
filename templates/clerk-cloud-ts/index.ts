import * as pulumi from "@pulumi/pulumi";
import * as clerk from "@pulumi/clerk";

const config = new pulumi.Config();
const apiKey = config.requireSecret("apiKey");
const templateName = config.get("templateName") ?? "access_token";
const claimsJson = config.get("claimsJson") ?? "{}";

// Explicit provider so credentials come from project-namespace config,
// not environment variables.
const provider = new clerk.Provider("clerk", { apiKey });

// Create a JWT template to customize claims on tokens issued by this Clerk instance.
const jwtTemplate = new clerk.JwtTemplate("jwt-template", {
    name: templateName,
    claims: claimsJson,
}, { provider });

// Export the template ID for downstream use (e.g. wiring into your application config).
export const templateId = jwtTemplate.id;
