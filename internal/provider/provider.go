// Package provider implements the FoPost Terraform provider. It is a thin
// declarative layer over the official Go SDK (github.com/fopost/fopost-go) —
// every request, retry, and error is the SDK's, never re-implemented here.
package provider

import (
	"context"
	"os"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	// APIKeyEnvVar is read when the provider block sets no api_key.
	APIKeyEnvVar = "FOPOST_API_KEY"
	// BaseURLEnvVar overrides the API root, for a staging or self-hosted deployment.
	BaseURLEnvVar = "FOPOST_BASE_URL"
)

var (
	_ provider.Provider = (*fopostProvider)(nil)
)

type fopostProvider struct {
	version string
}

// New builds the provider for a given release version, which travels in the
// User-Agent of every API call.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &fopostProvider{version: version}
	}
}

type providerModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func (p *fopostProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "fopost"
	resp.Version = p.version
}

func (p *fopostProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage [FoPost](https://fopost.com) as infrastructure: workspaces, labels, " +
			"webhooks, and automations. Posts are deliberately not managed here — see the provider guide below.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "A FoPost API key, created in the dashboard under Settings → API Keys. " +
					"Falls back to the `" + APIKeyEnvVar + "` environment variable, which is the recommended " +
					"place for it — a key committed to a `.tf` file is a leaked key.",
				Optional:  true,
				Sensitive: true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "The API root, including its version prefix. Falls back to the `" +
					BaseURLEnvVar + "` environment variable and then to `" + fopost.DefaultBaseURL +
					"`. Override it only to target a non-production FoPost deployment.",
				Optional: true,
			},
		},
	}
}

func (p *fopostProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown FoPost API Key",
			"The api_key argument cannot be resolved at plan time. Set it to a known value, or leave it "+
				"unset and export "+APIKeyEnvVar+" instead.",
		)
	}
	if config.BaseURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Unknown FoPost Base URL",
			"The base_url argument cannot be resolved at plan time. Set it to a known value, or leave it "+
				"unset and export "+BaseURLEnvVar+" instead.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := strings.TrimSpace(firstNonEmpty(str(config.APIKey), os.Getenv(APIKeyEnvVar)))
	baseURL := strings.TrimSpace(firstNonEmpty(str(config.BaseURL), os.Getenv(BaseURLEnvVar), fopost.DefaultBaseURL))

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing FoPost API Key",
			"The provider needs an API key to talk to the FoPost API. Set the api_key argument on the "+
				"provider block, or export "+APIKeyEnvVar+". Keys are created in the FoPost dashboard "+
				"under Settings → API Keys.",
		)
		return
	}

	// Keep the credential out of TF_LOG output.
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "api_key")
	tflog.Debug(ctx, "configuring the FoPost client", map[string]any{"base_url": baseURL})

	client, err := fopost.New(
		apiKey,
		fopost.WithBaseURL(baseURL),
		fopost.WithUserAgent("terraform-provider-fopost/"+p.version),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could Not Create the FoPost Client",
			"The FoPost SDK refused the provider configuration: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *fopostProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewWorkspaceResource,
		NewLabelResource,
		NewWebhookResource,
		NewAutomationResource,
	}
}

func (p *fopostProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewWorkspaceDataSource,
		NewWorkspacesDataSource,
		NewAccountDataSource,
		NewAccountsDataSource,
		NewLabelsDataSource,
	}
}
