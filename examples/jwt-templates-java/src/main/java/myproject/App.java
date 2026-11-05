package myproject;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.labs.clerk.JwtTemplate;
import com.pulumi.labs.clerk.JwtTemplateArgs;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var accessClaims = json(
            "user_id", "{{user.id}}",
            "email", "{{user.primary_email_address}}",
            "full_name", "{{user.full_name}}");
        var idClaims = json(
            "sub", "{{user.id}}",
            "email", "{{user.primary_email_address}}",
            "given_name", "{{user.first_name}}",
            "family_name", "{{user.last_name}}",
            "picture", "{{user.image_url}}");

        // Access token template -- adds user identity claims to short-lived access tokens.
        var accessTokenTemplate = new JwtTemplate("access-token", JwtTemplateArgs.builder()
            .name("access_token")
            .claims(accessClaims)
            .lifetime(3600)
            .allowedClockSkew(5)
            .build());

        // ID token template -- adds richer profile claims for display purposes.
        var idTokenTemplate = new JwtTemplate("id-token", JwtTemplateArgs.builder()
            .name("id_token")
            .claims(idClaims)
            .lifetime(3600)
            .build());

        ctx.export("accessTokenTemplateId", accessTokenTemplate.id());
        ctx.export("idTokenTemplateId", idTokenTemplate.id());
    }

    // Builds a flat JSON object from alternating keys and values (all strings).
    private static String json(String... pairs) {
        var sb = new StringBuilder("{");
        for (int i = 0; i < pairs.length; i += 2) {
            if (i > 0) {
                sb.append(",");
            }
            sb.append('"').append(pairs[i]).append("\":\"").append(pairs[i + 1]).append('"');
        }
        return sb.append("}").toString();
    }
}
