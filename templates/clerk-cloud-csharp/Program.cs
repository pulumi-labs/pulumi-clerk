using Pulumi;
using Clerk = Pulumi.Labs.Clerk;
using System.Collections.Generic;

return await Deployment.RunAsync(() =>
{
    var config = new global::Pulumi.Config();
    var templateName = config.Get("templateName") ?? "access_token";
    var claimsJson = config.Get("claimsJson") ?? "{}";

    // The default provider reads the key from `clerk:apiKey` or CLERK_API_KEY.

    // Create a JWT template to customize claims on tokens issued by this Clerk instance.
    var jwtTemplate = new Clerk.JwtTemplate("jwt-template", new Clerk.JwtTemplateArgs
    {
        Name = templateName,
        Claims = claimsJson,
    });

    // Export the template ID for downstream use.
    return new Dictionary<string, object?>
    {
        ["templateId"] = jwtTemplate.Id,
    };
});
