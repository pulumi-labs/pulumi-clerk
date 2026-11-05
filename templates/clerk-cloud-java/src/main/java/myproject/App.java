package myproject;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.labs.clerk.JwtTemplate;
import com.pulumi.labs.clerk.JwtTemplateArgs;
import com.pulumi.core.Output;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var config = ctx.config();
        var templateName = config.get("templateName").orElse("access_token");
        var claimsJson = config.get("claimsJson").orElse("{}");

        // The default provider reads the key from `clerk:apiKey` or CLERK_API_KEY.

        // Create a JWT template to customize claims on tokens issued by this Clerk instance.
        var jwtTemplate = new JwtTemplate("jwt-template", JwtTemplateArgs.builder()
            .name(templateName)
            .claims(claimsJson)
            .build());

        // Export the template ID for downstream use.
        ctx.export("templateId", jwtTemplate.id());
    }
}
