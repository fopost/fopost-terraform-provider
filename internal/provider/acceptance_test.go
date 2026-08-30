package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// The acceptance tests drive a real terraform binary against the in-memory
// fakeAPI, so they exercise plan, apply, refresh, import, and destroy without
// ever reaching the network. `resource.Test` skips them unless TF_ACC is set.

// testAccFake starts a fake FoPost API and points the provider at it.
func testAccFake(t *testing.T) *fakeAPI {
	t.Helper()
	api, server := newFakeAPI(t)
	t.Setenv(APIKeyEnvVar, fakeAPIKey)
	t.Setenv(BaseURLEnvVar, server.URL+"/v1")
	return api
}

// checkGone asserts the object left the fake API, so a destroy that silently
// did nothing fails the test.
func checkGone(store func() map[string]map[string]any) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if remaining := len(store()); remaining != 0 {
			return fmt.Errorf("destroy left %d objects behind", remaining)
		}
		return nil
	}
}

func TestAccWorkspace(t *testing.T) {
	api := testAccFake(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkGone(func() map[string]map[string]any { return api.workspaces }),
		Steps: []resource.TestStep{
			{
				Config: `
resource "fopost_workspace" "test" {
  name        = "Acme Social"
  slug        = "acme-social"
  type        = "TEAM"
  timezone    = "Europe/Berlin"
  description = "Everything Acme publishes."
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fopost_workspace.test", "id"),
					resource.TestCheckResourceAttr("fopost_workspace.test", "name", "Acme Social"),
					resource.TestCheckResourceAttr("fopost_workspace.test", "slug", "acme-social"),
					resource.TestCheckResourceAttr("fopost_workspace.test", "type", "TEAM"),
					resource.TestCheckResourceAttr("fopost_workspace.test", "timezone", "Europe/Berlin"),
					// The API defaults it; the provider must read the default back.
					resource.TestCheckResourceAttr("fopost_workspace.test", "language", "en"),
					resource.TestCheckResourceAttrSet("fopost_workspace.test", "created_at"),
				),
			},
			{
				Config: `
resource "fopost_workspace" "test" {
  name     = "Acme Social EU"
  slug     = "acme-social"
  type     = "TEAM"
  timezone = "Europe/Berlin"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fopost_workspace.test", "name", "Acme Social EU"),
					// Removing the argument clears it upstream instead of drifting.
					resource.TestCheckNoResourceAttr("fopost_workspace.test", "description"),
				),
			},
			{
				ResourceName:      "fopost_workspace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccWorkspaceDeletedOutsideTerraform pins the 404 contract: an object
// removed upstream is recreated, not an error.
func TestAccWorkspaceDeletedOutsideTerraform(t *testing.T) {
	api := testAccFake(t)

	config := `
resource "fopost_workspace" "test" {
  name = "Drift Test"
  slug = "drift-test"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkGone(func() map[string]map[string]any { return api.workspaces }),
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					for id := range api.workspaces {
						delete(api.workspaces, id)
					}
				},
				Config:             config,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccLabel(t *testing.T) {
	api := testAccFake(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkGone(func() map[string]map[string]any { return api.labels }),
		Steps: []resource.TestStep{
			{
				Config: `
resource "fopost_workspace" "test" {
  name = "Labels"
  slug = "labels"
}

resource "fopost_label" "launch" {
  workspace_id = fopost_workspace.test.id
  name         = "Launch"
  color        = "#2563eb"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fopost_label.launch", "name", "Launch"),
					resource.TestCheckResourceAttr("fopost_label.launch", "color", "#2563eb"),
					resource.TestCheckResourceAttrPair(
						"fopost_label.launch", "workspace_id",
						"fopost_workspace.test", "id",
					),
				),
			},
			{
				Config: `
resource "fopost_workspace" "test" {
  name = "Labels"
  slug = "labels"
}

resource "fopost_label" "launch" {
  workspace_id = fopost_workspace.test.id
  name         = "Launch Week"
  color        = "#16a34a"
}
`,
				Check: resource.TestCheckResourceAttr("fopost_label.launch", "name", "Launch Week"),
			},
			{
				ResourceName:      "fopost_label.launch",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccWebhook(t *testing.T) {
	api := testAccFake(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkGone(func() map[string]map[string]any { return api.webhooks }),
		Steps: []resource.TestStep{
			{
				Config: `
resource "fopost_workspace" "test" {
  name = "Hooks"
  slug = "hooks"
}

resource "fopost_webhook" "delivery" {
  workspace_id = fopost_workspace.test.id
  url          = "https://hooks.example.com/fopost"
  events       = ["post.published", "delivery.failed"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fopost_webhook.delivery", "url", "https://hooks.example.com/fopost"),
					resource.TestCheckResourceAttr("fopost_webhook.delivery", "events.#", "2"),
					resource.TestCheckResourceAttr("fopost_webhook.delivery", "active", "true"),
					resource.TestCheckResourceAttr("fopost_webhook.delivery", "secret", "whsec_fixture"),
				),
			},
			{
				Config: `
resource "fopost_workspace" "test" {
  name = "Hooks"
  slug = "hooks"
}

resource "fopost_webhook" "delivery" {
  workspace_id = fopost_workspace.test.id
  url          = "https://hooks.example.com/fopost/v2"
  events       = ["post.published"]
  active       = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fopost_webhook.delivery", "url", "https://hooks.example.com/fopost/v2"),
					resource.TestCheckResourceAttr("fopost_webhook.delivery", "events.#", "1"),
					resource.TestCheckResourceAttr("fopost_webhook.delivery", "active", "false"),
					// The secret is shown once, at creation, and must survive an update.
					resource.TestCheckResourceAttr("fopost_webhook.delivery", "secret", "whsec_fixture"),
				),
			},
			{
				ResourceName:      "fopost_webhook.delivery",
				ImportState:       true,
				ImportStateVerify: true,
				// The API never re-issues the signing secret, so an imported
				// subscription legitimately has none.
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

func TestAccAutomation(t *testing.T) {
	api := testAccFake(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkGone(func() map[string]map[string]any { return api.automations }),
		Steps: []resource.TestStep{
			{
				Config: `
resource "fopost_workspace" "test" {
  name = "Automations"
  slug = "automations"
}

resource "fopost_automation" "feed" {
  workspace_id   = fopost_workspace.test.id
  name           = "Blog to social"
  trigger_type   = "rss_feed"
  trigger_config = jsonencode({ feed_url = "https://example.com/feed.xml" })

  step {
    action_type   = "delay"
    action_config = jsonencode({ minutes = 15 })
  }

  step {
    action_type = "publish"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fopost_automation.feed", "name", "Blog to social"),
					resource.TestCheckResourceAttr("fopost_automation.feed", "trigger_type", "rss_feed"),
					resource.TestCheckResourceAttr("fopost_automation.feed", "active", "true"),
					resource.TestCheckResourceAttr("fopost_automation.feed", "step.#", "2"),
					// The API numbers the steps from the order they are declared in.
					resource.TestCheckResourceAttr("fopost_automation.feed", "step.0.position", "1"),
					resource.TestCheckResourceAttr("fopost_automation.feed", "step.0.action_type", "delay"),
					resource.TestCheckResourceAttr("fopost_automation.feed", "step.1.position", "2"),
					resource.TestCheckResourceAttr("fopost_automation.feed", "step.1.action_type", "publish"),
				),
			},
			{
				Config: `
resource "fopost_workspace" "test" {
  name = "Automations"
  slug = "automations"
}

resource "fopost_automation" "feed" {
  workspace_id   = fopost_workspace.test.id
  name           = "Blog to social"
  trigger_type   = "rss_feed"
  trigger_config = jsonencode({ feed_url = "https://example.com/feed.xml" })
  active         = false

  step {
    action_type = "publish"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fopost_automation.feed", "active", "false"),
					resource.TestCheckResourceAttr("fopost_automation.feed", "step.#", "1"),
				),
			},
			{
				ResourceName:            "fopost_automation.feed",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

// TestAccAutomationTriggerTypeForcesReplacement pins the immutable attribute:
// the API cannot move an automation to another trigger, so Terraform must
// replace it rather than send an update that silently does nothing.
func TestAccAutomationTriggerTypeForcesReplacement(t *testing.T) {
	testAccFake(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "fopost_automation" "feed" {
  workspace_id = "ws_external"
  name         = "Trigger swap"
  trigger_type = "rss_feed"

  step {
    action_type = "publish"
  }
}
`,
			},
			{
				Config: `
resource "fopost_automation" "feed" {
  workspace_id = "ws_external"
  name         = "Trigger swap"
  trigger_type = "schedule"

  step {
    action_type = "publish"
  }
}
`,
				Check: resource.TestCheckResourceAttr("fopost_automation.feed", "trigger_type", "schedule"),
			},
		},
	})
}

func TestAccDataSources(t *testing.T) {
	api := testAccFake(t)
	api.mu.Lock()
	api.workspaces["ws_fixture"] = map[string]any{
		"id": "ws_fixture", "name": "Fixture", "slug": "fixture", "type": "TEAM",
		"timezone": "UTC", "language": "en", "accounts": []any{},
		"created_at": "2026-08-30T09:00:00Z", "updated_at": "2026-08-30T09:00:00Z",
	}
	api.labels["lbl_fixture"] = map[string]any{
		"id": "lbl_fixture", "name": "Evergreen", "color": "#2563eb",
		"workspace":  map[string]any{"id": "ws_fixture", "name": "Fixture", "slug": "fixture", "type": "TEAM"},
		"created_at": "2026-08-30T09:00:00Z", "updated_at": "2026-08-30T09:00:00Z",
	}
	api.mu.Unlock()
	accountID := api.addAccount("ws_fixture", "linkedin", "acme")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "fopost_workspace" "one" {
  id = "ws_fixture"
}

data "fopost_workspaces" "all" {}

data "fopost_account" "one" {
  id = %q
}

data "fopost_accounts" "all" {
  workspace_id = "ws_fixture"
}

data "fopost_labels" "all" {
  workspace_id = "ws_fixture"
}
`, accountID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.fopost_workspace.one", "name", "Fixture"),
					resource.TestCheckResourceAttr("data.fopost_workspace.one", "type", "TEAM"),
					resource.TestCheckResourceAttr("data.fopost_workspaces.all", "workspaces.#", "1"),
					resource.TestCheckResourceAttr("data.fopost_account.one", "platform", "linkedin"),
					resource.TestCheckResourceAttr("data.fopost_account.one", "workspace_slug", "fixture"),
					resource.TestCheckResourceAttr("data.fopost_accounts.all", "accounts.#", "1"),
					resource.TestCheckResourceAttr("data.fopost_accounts.all", "accounts.0.health_status", "healthy"),
					resource.TestCheckResourceAttr("data.fopost_labels.all", "labels.#", "1"),
					resource.TestCheckResourceAttr("data.fopost_labels.all", "labels.0.name", "Evergreen"),
				),
			},
		},
	})
}
