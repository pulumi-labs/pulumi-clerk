import json
import pulumi
import pulumi_clerk as clerk

config = pulumi.Config()
api_key = config.require_secret("apiKey")

# Explicit provider so credentials come from project-namespace config,
# not environment variables.
provider = clerk.Provider("clerk", api_key=api_key)

# Access token template -- adds user identity claims to short-lived access tokens.
access_token_template = clerk.JwtTemplate("access-token",
    name="access_token",
    claims=json.dumps({
        "user_id": "{{user.id}}",
        "email": "{{user.primary_email_address}}",
        "full_name": "{{user.full_name}}",
    }),
    lifetime=3600,
    allowed_clock_skew=5,
    opts=pulumi.ResourceOptions(provider=provider),
)

# ID token template -- adds richer profile claims for display purposes.
id_token_template = clerk.JwtTemplate("id-token",
    name="id_token",
    claims=json.dumps({
        "sub": "{{user.id}}",
        "email": "{{user.primary_email_address}}",
        "given_name": "{{user.first_name}}",
        "family_name": "{{user.last_name}}",
        "picture": "{{user.image_url}}",
    }),
    lifetime=3600,
    opts=pulumi.ResourceOptions(provider=provider),
)

pulumi.export("accessTokenTemplateId", access_token_template.id)
pulumi.export("idTokenTemplateId", id_token_template.id)
