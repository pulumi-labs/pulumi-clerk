---
title: Clerk Installation & Configuration
meta_desc: How to install the Pulumi Clerk provider and its language SDKs, and how to configure the instance secret key and the Platform API key.
layout: package
---

## Installation

The Clerk provider is available as a package in all Pulumi-supported languages:

* JavaScript/TypeScript: [`@pulumi-labs/clerk`](https://www.npmjs.com/package/@pulumi-labs/clerk)
* Python: [`pulumi-labs-clerk`](https://pypi.org/project/pulumi-labs-clerk/)
* Go: [`github.com/pulumi-labs/pulumi-clerk/sdk/go/clerk`](https://github.com/pulumi-labs/pulumi-clerk)
* .NET: [`Pulumi.Labs.Clerk`](https://www.nuget.org/packages/Pulumi.Labs.Clerk)
* Java: [`com.pulumi.labs:clerk`](https://central.sonatype.com/artifact/com.pulumi.labs/clerk)

In normal usage, installing one of the language SDKs above is enough. The first `pulumi up` automatically downloads the matching provider binary from the URL embedded in the SDK, so no separate step is required.

### Provider Binary (optional)

If you need to install the provider binary explicitly (for example, in an air-gapped environment or to prewarm a CI cache), use the `pulumi plugin` command:

```bash
pulumi plugin install resource clerk <version> --server github://api.github.com/pulumi-labs/pulumi-clerk
```

Replace `<version>` with your desired release (see the [releases page](https://github.com/pulumi-labs/pulumi-clerk/releases)).

## Configuration

The provider authenticates with the Clerk instance secret key. It needs no explicit `Provider` resource and no stack config when the key is available as an environment variable:

```bash
export CLERK_API_KEY=<instance-secret-key>
```

You can also set it as provider configuration:

```bash
pulumi config set --secret clerk:apiKey <instance-secret-key>
```

Explicit Pulumi config wins over the environment variable. The following keys are available:

| Key | Env var | Default | Description |
|---|---|---|---|
| `apiKey` (secret) | `CLERK_API_KEY`, then `CLERK_SECRET_KEY` | none | Clerk instance secret key (starts with `sk_test_` or `sk_live_`). Required by every resource except Application, Domain, InstanceDomain and InstanceConfig. |
| `platformApiKey` (secret) | `CLERK_PLATFORM_API_KEY` | none | Clerk Platform API key (beta). Required by Application, Domain, InstanceDomain and InstanceConfig. |
| `apiUrl` | `CLERK_API_URL` | `https://api.clerk.com/v1` | Override for the Clerk API base URL. Only needed for tests and proxies. |

An explicit `clerk` provider resource is only needed to target several Clerk instances in one program.
