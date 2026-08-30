# Terraform Provider for FoPost

[![CI](https://github.com/fopost/terraform-provider-fopost/actions/workflows/ci.yml/badge.svg)](https://github.com/fopost/terraform-provider-fopost/actions/workflows/ci.yml)
[![Terraform Registry](https://img.shields.io/badge/registry-fopost%2Ffopost-7B42BC)](https://registry.terraform.io/providers/fopost/fopost/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

The official Terraform provider for [FoPost](https://fopost.com) — manage the durable parts
of your social publishing setup as code: workspaces, labels, webhooks, and automations.

Built on [terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework)
and the official [FoPost Go SDK](https://github.com/fopost/fopost-go), which owns every
request, retry, and error the provider makes.

## Install

```hcl
terraform {
  required_providers {
    fopost = {
      source  = "fopost/fopost"
      version = "~> 0.1"
    }
  }
}
```

Requires Terraform 1.8 or newer.

## Configure

The provider needs a FoPost API key, created in the dashboard under **Settings → API Keys**.

```bash
export FOPOST_API_KEY="fp_..."
```

```hcl
provider "fopost" {}
```

Or pass it explicitly, from a variable your secret store fills in:

```hcl
provider "fopost" {
  api_key = var.fopost_api_key
}
```

| Argument   | Environment variable | Default                     |
| ---------- | -------------------- | --------------------------- |
| `api_key`  | `FOPOST_API_KEY`     | —                           |
| `base_url` | `FOPOST_BASE_URL`    | `https://api.fopost.com/v1` |

`api_key` is marked sensitive, so it never appears in plan output. Leave `base_url` alone
unless you are targeting a non-production FoPost deployment.

## Example

A workspace, the labels its campaigns report against, and a webhook that pushes delivery
events to a system you operate:

```hcl
resource "fopost_workspace" "acme" {
  name        = "Acme Social"
  slug        = "acme-social"
  type        = "TEAM"
  timezone    = "Europe/Berlin"
  description = "Everything Acme publishes."
}

resource "fopost_label" "launch" {
  workspace_id = fopost_workspace.acme.id
  name         = "Launch Week"
  color        = "#2563eb"
}

resource "fopost_webhook" "delivery" {
  workspace_id = fopost_workspace.acme.id
  url          = "https://hooks.acme.example.com/fopost"

  events = [
    "post.published",
    "post.failed",
    "delivery.failed",
    "account.health_changed",
  ]
}

# Issued once, at creation. Hand it to whatever verifies the signature.
output "webhook_signing_secret" {
  value     = fopost_webhook.delivery.secret
  sensitive = true
}
```

A longer configuration, including an RSS automation and the account data sources, is in
[`examples/complete`](examples/complete).

## What It Manages

| Resource            | What it is                                                   |
| ------------------- | ------------------------------------------------------------ |
| `fopost_workspace`  | The tenant boundary everything else is scoped to             |
| `fopost_label`      | A campaign tag posts are grouped and reported by             |
| `fopost_webhook`    | An outbound event subscription, with its signing secret      |
| `fopost_automation` | A trigger and the ordered steps it runs                      |

| Data source         | What it reads                                                |
| ------------------- | ------------------------------------------------------------ |
| `fopost_workspace`  | One workspace and the accounts connected to it               |
| `fopost_workspaces` | Every workspace the API key can reach                        |
| `fopost_account`    | One connected social account                                 |
| `fopost_accounts`   | Connected accounts and their connection health               |
| `fopost_labels`     | Labels, optionally narrowed to one workspace                 |

Every resource supports `terraform import` by its identifier.

**Posts are deliberately not a resource.** A published post is an event, not infrastructure:
Terraform would try to "update" content already live on a social network, and would delete
real posts the moment the block left the configuration. Create posts through the
[FoPost API](https://fopost.com/docs) or one of the SDKs. **Connected accounts are read, not
created** — connecting one is an interactive OAuth handshake in the dashboard.

## Errors, Retries, and Drift

Transport is the Go SDK's, unchanged: a 30-second timeout, three attempts, exponential
backoff on `429` and `5xx`, and `Retry-After` honoured. The provider translates what is left
into a diagnostic that names the failed action, the HTTP status, and the API's own error
code — never the API key.

Reads treat a `404` as "gone upstream": the object is removed from state and planned for
re-creation rather than failing the run.

## Local Development

The provider is a Go binary. Build and install it, then tell Terraform to use your build
instead of the registry:

```bash
go install .
```

`~/.terraformrc` (`%APPDATA%\terraform.rc` on Windows):

```hcl
provider_installation {
  dev_overrides {
    "fopost/fopost" = "/Users/you/go/bin"
  }

  direct {}
}
```

With an override in place, skip `terraform init` and run `terraform plan` directly —
Terraform will print a warning saying it is bypassing installation, which is expected.

```bash
make build       # go build ./...
make test        # unit tests, offline
make testacc     # acceptance tests against an in-process fake API (needs terraform on PATH)
make lint        # gofmt + go vet
make docs        # regenerate docs/ from the schemas and examples/
```

The acceptance tests never touch the real FoPost API — they drive a real `terraform` binary
against an `httptest` server that stands in for it, so they need no credentials.

To attach a debugger, run `go run . -debug` and export the `TF_REATTACH_PROVIDERS` value it
prints.

## Documentation and Support

- Provider reference: [registry.terraform.io/providers/fopost/fopost](https://registry.terraform.io/providers/fopost/fopost/latest/docs)
- FoPost API and SDKs: [fopost.com/docs](https://fopost.com/docs)
- Questions and bugs: [GitHub issues](https://github.com/fopost/terraform-provider-fopost/issues)
- Anything else: [fopost.com/contact](https://fopost.com/contact)

## License

MIT. Copyright (c) 2026 Porter Bridge, LLC.
