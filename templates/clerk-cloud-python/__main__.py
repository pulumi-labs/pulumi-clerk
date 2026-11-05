import pulumi
import pulumi_labs_clerk as clerk

config = pulumi.Config()
template_name = config.get("templateName") or "access_token"
claims_json = config.get("claimsJson") or "{}"

# The default provider reads the key from `clerk:apiKey` or CLERK_API_KEY.

# Create a JWT template to customize claims on tokens issued by this Clerk instance.
jwt_template = clerk.JwtTemplate("jwt-template",
    name=template_name,
    claims=claims_json,
)

# Export the template ID for downstream use.
pulumi.export("templateId", jwt_template.id)
