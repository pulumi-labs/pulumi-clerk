package myproject;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.clerk.JwtTemplate;
import com.pulumi.clerk.JwtTemplateArgs;
import com.pulumi.clerk.Provider;
import com.pulumi.clerk.ProviderArgs;
import com.pulumi.core.Output;
import com.pulumi.resources.CustomResourceOptions;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var config = ctx.config();
        var apiKey = config.requireSecret("apiKey");
        var templateName = config.get("templateName").orElse("access_token");
        var claimsJson = config.get("claimsJson").orElse("{}");

        // Explicit provider so credentials come from project-namespace config,
        // not environment variables.
        var provider = new Provider("clerk", ProviderArgs.builder()
            .apiKey(apiKey)
            .build());

        // Create a JWT template to customize claims on tokens issued by this Clerk instance.
        var jwtTemplate = new JwtTemplate("jwt-template", JwtTemplateArgs.builder()
            .name(templateName)
            .claims(claimsJson)
            .build(),
            CustomResourceOptions.builder()
                .provider(provider)
                .build());

        // Export the template ID for downstream use.
        ctx.export("templateId", jwtTemplate.id());
    }
}
