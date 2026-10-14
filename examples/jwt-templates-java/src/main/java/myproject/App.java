package myproject;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.clerk.JwtTemplate;
import com.pulumi.clerk.JwtTemplateArgs;
import java.util.Map;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var mapper = new ObjectMapper();

        String accessClaims;
        String idClaims;
        try {
            accessClaims = mapper.writeValueAsString(Map.of(
                "user_id", "{{user.id}}",
                "email", "{{user.primary_email_address}}",
                "full_name", "{{user.full_name}}"
            ));
            idClaims = mapper.writeValueAsString(Map.of(
                "sub", "{{user.id}}",
                "email", "{{user.primary_email_address}}",
                "given_name", "{{user.first_name}}",
                "family_name", "{{user.last_name}}",
                "picture", "{{user.image_url}}"
            ));
        } catch (Exception e) {
            throw new RuntimeException("Failed to serialize JWT claims", e);
        }

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
}
