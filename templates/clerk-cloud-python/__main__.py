import pulumi
import pulumi_clerk as clerk

config = pulumi.Config()
api_key = config.require_secret("apiKey")
template_name = config.get("templateName") or "access_token"
claims_json = config.get("claimsJson") or "{}"

# Explicit provider so credentials come from project-namespace config,
# not environment variables.
provider = clerk.Provider("clerk", api_key=api_key)

# Create a JWT template to customize claims on tokens issued by this Clerk instance.
jwt_template = clerk.JwtTemplate("jwt-template",
    name=template_name,
    claims=claims_json,
    opts=pulumi.ResourceOptions(provider=provider),
)

# Export the template ID for downstream use.
pulumi.export("templateId", jwt_template.id)
