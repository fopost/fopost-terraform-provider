# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

## What This Is

`terraform-provider-fopost` — the official Terraform provider for
[FoPost](https://fopost.com), published to the Terraform Registry as `fopost/fopost`. It is
built on **terraform-plugin-framework v1.x** (not the legacy SDKv2) and is a *consumer* of
the official Go SDK, `github.com/fopost/fopost-go`. Version `0.1.0`.

The provider owns schemas, plan behaviour, state, and diagnostics. **It owns no transport.**
HTTP, authentication headers, retries, backoff, `Retry-After`, envelope unwrapping, and
error typing all live in the SDK. If you find yourself writing an `http.Client` here, stop —
the answer is a method on the SDK, or the SDK's `client.Do` escape hatch.

## ⚠️ Repository Name Must Change Before Publishing

**The Terraform Registry requires the GitHub repository be named `terraform-provider-<name>`.**
This repository is `fopost/fopost-terraform-provider`, which does **not** match. The registry
publish flow will reject it.

Everything inside the repo already uses the conventional name — the Go module is
`github.com/fopost/terraform-provider-fopost`, the binary is `terraform-provider-fopost`, and
`.goreleaser.yml` names artifacts from it. Only the GitHub repository name is wrong.

Resolve it before the first publish, in one of two ways:

1. **Rename the GitHub repository to `terraform-provider-fopost`.** Preferred. GitHub keeps a
   redirect from the old name, and the Go module path already matches, so nothing else moves.
2. **Publish from a mirror repository** named `terraform-provider-fopost`, kept in sync from
   this one.

Do not "fix" this by renaming the Go module to match the directory — the module path is
correct as it stands, and `registry.terraform.io/fopost/fopost` in `main.go` is what the
registry resolves against.

## Brand Rules

- The product is **FoPost** (`fopost.com`). Never write "OwlStack" — retired Aug 2026.
- Never write an email address. Support is https://fopost.com/contact and GitHub issues.
- Never name AI providers/models, infrastructure vendors, or any person.

## Scope: What Is a Resource, and What Deliberately Is Not

This is the decision most likely to be "helpfully" undone. Read it before adding anything.

**Terraform models durable configuration** — objects with a stable identity that should keep
existing until the configuration says otherwise, and whose deletion on `terraform destroy` is
the correct behaviour.

Managed resources:

| Resource | Why |
| --- | --- |
| `fopost_workspace` | The tenant boundary. Long-lived; everything else hangs off it. |
| `fopost_label` | Campaign taxonomy. Worth keeping identical across environments. |
| `fopost_webhook` | An integration contract with a system the same team operates. |
| `fopost_automation` | A standing rule. Reviewing a change to it in a PR is exactly the point. |

Data sources: `fopost_workspace`, `fopost_workspaces`, `fopost_account`, `fopost_accounts`,
`fopost_labels`.

**`fopost_post` must never exist.** A published post is an *event*, not infrastructure.
Modelling it as a resource would mean:

- Terraform planning an "update" to content that is already live on a social network, where
  most platforms cannot edit and the SDK's update only touches a draft.
- `terraform destroy`, or simply deleting the block, **deleting real published posts**.
- Perpetual diffs from every field the platform writes back after publishing.

The same reasoning rules out `media`, `analytics`, and `automation_run`: uploads and runs are
events, analytics is a read of the past. If a caller wants scheduled content in code, the
answer is the API, an SDK, or a `null_resource`-style one-shot in their own tooling — not a
managed resource here. `internal/provider/provider_test.go` pins the resource list, so adding
one fails a test until the decision is made deliberately.

**Connected accounts are read-only.** Connecting one is an interactive OAuth handshake with
the platform. `AccountsService.Create` exists in the SDK for credential-based platforms, but
storing platform credentials in Terraform state is the wrong shape, so it is not exposed.

## Architecture

```
main.go                       providerserver.Serve, address registry.terraform.io/fopost/fopost, -debug flag
internal/provider/
  provider.go                 provider schema, Configure (api_key/base_url resolution), registries
  client.go                   ProviderData → *fopost.Client, importIDPath, firstNonEmpty
  errors.go                   SDK error → Terraform diagnostic; isNotFound
  convert.go                  API value → Terraform value helpers (and back)
  resource_*.go               one file per managed resource
  data_source_*.go            one file per data source
  fake_api_test.go            in-memory FoPost API for the acceptance tests
  acceptance_test.go          resource.Test lifecycles, gated by TF_ACC
examples/                     source of the "Example Usage" blocks in docs/
templates/index.md.tmpl       custom provider landing page (auth, scope, dev_overrides)
docs/                         GENERATED by tfplugindocs — never hand-edit
```

A request flows: Terraform → resource `Create`/`Read`/`Update`/`Delete` → `r.client.<Service>`
(the SDK) → HTTP. Errors come back as `*fopost.Error` and go through `apiDiagnostic`.

### Rules that keep the provider correct

- **Read must handle 404 by removing the resource from state**, never by erroring. Otherwise
  an object deleted in the dashboard wedges every future plan. `isNotFound(err)` is the check.
  `fopost_webhook` has no per-id read endpoint, so its `Read` lists and treats "absent from the
  list" the same way.
- **Immutable attributes get `RequiresReplace`.** Anything the SDK's `Update*Request` cannot
  carry is immutable: `workspace_id` on label, webhook, and automation; `trigger_type` on
  automation. Sending an update that silently does nothing is worse than replacing.
- **Server-assigned fields are `Computed`**, with `UseStateForUnknown()` on the stable ones
  (`id`, `created_at`, `secret`) so they do not read as "known after apply" on every change.
  Do not add it to genuinely drifting values like `run_count` or `last_triggered_at`.
- **Server-defaulted fields are `Optional + Computed`** (`type`, `timezone`, `language`,
  `trigger_config`, `action_config`) so the API's default can land in state.
- **Secrets are issued once.** `fopost_webhook.secret` and `fopost_automation.secret` come back
  only from the create call. `Read` and `Update` must preserve the state value rather than
  writing the null the API returns; an imported resource legitimately has none, which is why
  the import tests carry `ImportStateVerifyIgnore: []string{"secret"}`.
- **Never put the API key in a diagnostic.** `TestConfigureNeverLeaksTheAPIKey` pins it, and
  `Configure` masks the field for `tflog`.
- **Free-form JSON config objects** (`trigger_config`, `action_config`) use
  `jsontypes.Normalized`, which compares semantically so key order and whitespace never
  produce a diff. Do not swap it for `types.Map` — the API values are not uniformly typed.

## API Contract

- Base URL `https://api.fopost.com/v1`, overridable with `FOPOST_BASE_URL`.
- Auth header `X-API-Key` (**not** Bearer), from `api_key` or `FOPOST_API_KEY`.
- Most responses are wrapped in `{"data": ...}`; the SDK unwraps it.
- Errors are `{"error": "<code>", "message": "<text>"}`; `402` may carry `upgrade_url`.
- Retries: 3 attempts total, exponential backoff from 500 ms, only on `429` and `5xx` and
  network errors, honouring `Retry-After`. All of this is the SDK's — do not re-implement.

## Commands

```bash
make build       # go build ./...
make test        # unit tests, fully offline
make testacc     # TF_ACC=1 acceptance tests; needs a terraform binary on PATH
make lint        # gofmt -l . && go vet ./...
make docs        # regenerate docs/ with tfplugindocs; needs a terraform binary
go install .     # for dev_overrides
go run . -debug  # run under a debugger; exports TF_REATTACH_PROVIDERS
```

The acceptance tests drive a real `terraform` binary against `fake_api_test.go`, an in-memory
stand-in for the FoPost API. They never reach the network and need no credentials. If
`terraform` is not on PATH, the test harness downloads one; where that download is blocked,
put a binary on PATH first.

**Go version.** `go.mod` targets the Go version `terraform-plugin-testing` requires, which is
well above the 1.22 floor the other FoPost SDKs use. That is the toolchain's constraint, not a
choice — lowering it means pinning an old plugin-framework release. Let `go mod tidy` decide it.

## Parent Dependency

`github.com/fopost/fopost-go` is resolved from the Go module proxy. It has **no `v0.1.0` tag
yet**, so `go.mod` currently pins the pseudo-version of its `main` tip. Once `fopost-go` tags
its release, bump this to `v0.1.0` — the pseudo-version is a green-build shim, not the
intended coordinate.

## Conventions

- `gofmt` on everything; Prettier-style prose in Markdown is irrelevant here.
- Comments are short and explain a *why*. No narrated docblocks over obvious code. Exported
  symbols carry a doc comment.
- Every attribute needs a `MarkdownDescription` and every resource and data source needs a
  schema-level one — `provider_test.go` fails otherwise, because a missing description means a
  blank row on the registry page.
- Every new resource or data source ships with: an entry in `Resources()`/`DataSources()`, an
  `examples/{resources,data-sources}/fopost_<name>/` file, regenerated `docs/`, and — for a
  resource — `ImportState` plus an acceptance test covering create, update, and import.
- `docs/` is generated. Change a description in the schema and run `make docs`; never edit the
  Markdown. CI fails on drift.

## Releasing

Tag `v<version>`; `.github/workflows/release.yml` runs GoReleaser, which cross-compiles for
every platform the registry expects, writes `terraform-provider-fopost_<version>_SHA256SUMS`,
and GPG-signs it.

Requires repository secrets:

- `GPG_PRIVATE_KEY` — the ASCII-armored private key that signs the checksums.
- `PASSPHRASE` — its passphrase.

### Terraform Registry onboarding (first publish, done once, by hand)

1. **Rename the repository to `terraform-provider-fopost`** — see the warning at the top. The
   registry will not accept it otherwise.
2. **Generate a GPG signing key** for the `fopost` namespace and keep it out of this repo. Add
   the private key and passphrase as the two secrets above.
3. **Register the public key** with the Terraform Registry, under the `fopost` organisation's
   signing keys. The registry verifies every release's `SHA256SUMS.sig` against it; a key
   registered after a release does not retroactively validate it.
4. **Claim the provider** at [registry.terraform.io](https://registry.terraform.io) →
   *Publish* → *Provider*, signed in as a GitHub user with admin on the repo. This installs a
   webhook; it is a manual, one-time step no workflow can do.
5. **Push the first tag** (`v0.1.0`). The release workflow builds and signs; the registry picks
   the release up through the webhook, usually within minutes.
6. Later versions need only the tag. `terraform-registry-manifest.json` declares protocol
   `6.0` and must stay in the release artifacts — GoReleaser attaches it.

## Git

Conventional Commits, atomic. Branch `feature/<description>`, merge to `main` via PR.
Never `gh pr create` — push the branch and hand over the compare link.
