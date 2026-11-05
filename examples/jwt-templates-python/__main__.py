import json
import pulumi
import pulumi_labs_clerk as clerk

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
)

pulumi.export("accessTokenTemplateId", access_token_template.id)
pulumi.export("idTokenTemplateId", id_token_template.id)
