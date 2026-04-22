using Pulumi;
using Clerk = Pulumi.Clerk;
using System.Collections.Generic;

return await Deployment.RunAsync(() =>
{
    var config = new global::Pulumi.Config();
    var apiKey = config.RequireSecret("apiKey");
    var templateName = config.Get("templateName") ?? "access_token";
    var claimsJson = config.Get("claimsJson") ?? "{}";

    // Explicit provider so credentials come from project-namespace config,
    // not environment variables.
    var provider = new Clerk.Provider("clerk", new Clerk.ProviderArgs
    {
        ApiKey = apiKey,
    });

    // Create a JWT template to customize claims on tokens issued by this Clerk instance.
    var jwtTemplate = new Clerk.JwtTemplate("jwt-template", new Clerk.JwtTemplateArgs
    {
        Name = templateName,
        Claims = claimsJson,
    }, new CustomResourceOptions { Provider = provider });

    // Export the template ID for downstream use.
    return new Dictionary<string, object?>
    {
        ["templateId"] = jwtTemplate.Id,
    };
});
