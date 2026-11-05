---
title: Clerk Provider
meta_desc: Manage Clerk allowlists, blocklists, JWT templates, organizations, roles and permissions, OAuth applications, SAML connections, API keys, machines and webhooks with Pulumi.
layout: package
---

An experimental Pulumi provider for [Clerk](https://clerk.com), the authentication and user management platform. It manages Clerk configuration as infrastructure: allowlists, blocklists, JWT templates, organizations and their roles and permissions, OAuth applications, SAML connections, API keys, machines and webhooks. Applications, domains and instance configuration are available through the Clerk Platform API (beta).

## Installation

The Clerk provider is available as a package in all Pulumi-supported languages:

* JavaScript/TypeScript: [`@pulumi-labs/clerk`](https://www.npmjs.com/package/@pulumi-labs/clerk)
* Python: [`pulumi-labs-clerk`](https://pypi.org/project/pulumi-labs-clerk/)
* Go: [`github.com/pulumi-labs/pulumi-clerk/sdk/go/clerk`](https://github.com/pulumi-labs/pulumi-clerk)
* .NET: [`Pulumi.Labs.Clerk`](https://www.nuget.org/packages/Pulumi.Labs.Clerk)
* Java: [`com.pulumi.labs:clerk`](https://central.sonatype.com/artifact/com.pulumi.labs/clerk)

## Example Usage

{{< chooser language "typescript,python,yaml" >}}
{{% choosable language typescript %}}
```typescript
import * as clerk from "@pulumi-labs/clerk";

const template = new clerk.JwtTemplate("supabase", {
    name: "supabase",
    claims: JSON.stringify({ role: "authenticated" }),
});

export const jwtTemplateId = template.id;
```
{{% /choosable %}}
{{% choosable language python %}}
```python
import pulumi
import pulumi_labs_clerk as clerk
import json

template = clerk.JwtTemplate(
    "supabase",
    name="supabase",
    claims=json.dumps({"role": "authenticated"}),
)

pulumi.export("jwtTemplateId", template.id)
```
{{% /choosable %}}
{{% choosable language yaml %}}
```yaml
resources:
  supabase:
    type: clerk:index:JwtTemplate
    properties:
      name: supabase
      claims: '{"role": "authenticated"}'
outputs:
  jwtTemplateId: ${supabase.id}
```
{{% /choosable %}}
{{< /chooser >}}

For runnable programs in every language, see the [`examples/`](https://github.com/pulumi-labs/pulumi-clerk/tree/main/examples) directory in the provider repository.

## Configuration

The provider works with no explicit `Provider` resource. Set the instance secret key as an environment variable and every resource picks it up. See Installation & Configuration for the other ways to configure credentials.
