using Pulumi;
using Clerk = Pulumi.Clerk;
using System.Collections.Generic;
using System.Text.Json;

return await Deployment.RunAsync(() =>
{
    // Access token template -- adds user identity claims.
    var accessClaims = JsonSerializer.Serialize(new
    {
        user_id = "{{user.id}}",
        email = "{{user.primary_email_address}}",
        full_name = "{{user.full_name}}",
    });
    var accessTokenTemplate = new Clerk.JwtTemplate("access-token", new Clerk.JwtTemplateArgs
    {
        Name = "access_token",
        Claims = accessClaims,
        Lifetime = 3600,
        AllowedClockSkew = 5,
    });

    // ID token template -- adds richer profile claims for display purposes.
    var idClaims = JsonSerializer.Serialize(new
    {
        sub = "{{user.id}}",
        email = "{{user.primary_email_address}}",
        given_name = "{{user.first_name}}",
        family_name = "{{user.last_name}}",
        picture = "{{user.image_url}}",
    });
    var idTokenTemplate = new Clerk.JwtTemplate("id-token", new Clerk.JwtTemplateArgs
    {
        Name = "id_token",
        Claims = idClaims,
        Lifetime = 3600,
    });

    return new Dictionary<string, object?>
    {
        ["accessTokenTemplateId"] = accessTokenTemplate.Id,
        ["idTokenTemplateId"] = idTokenTemplate.Id,
    };
});
