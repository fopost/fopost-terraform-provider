package provider

import (
	"context"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*accountDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*accountDataSource)(nil)
)

// NewAccountDataSource reads one connected social account by id.
func NewAccountDataSource() datasource.DataSource { return &accountDataSource{} }

type accountDataSource struct {
	client *fopost.Client
}

type accountDataModel struct {
	ID            types.String `tfsdk:"id"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	Platform      types.String `tfsdk:"platform"`
	Username      types.String `tfsdk:"username"`
	Name          types.String `tfsdk:"name"`
	Avatar        types.String `tfsdk:"avatar"`
	WorkspaceName types.String `tfsdk:"workspace_name"`
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	WorkspaceType types.String `tfsdk:"workspace_type"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (d *accountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (d *accountDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one connected social account. Accounts are connected in the FoPost " +
			"dashboard through each platform's OAuth flow, so they are read here, never created.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the account to read.",
				Required:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the workspace the account belongs to.",
				Computed:            true,
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "The social network the account is on, e.g. `linkedin`.",
				Computed:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Handle on the platform.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name on the platform.",
				Computed:            true,
			},
			"avatar": schema.StringAttribute{
				MarkdownDescription: "URL of the account's avatar.",
				Computed:            true,
			},
			"workspace_name": schema.StringAttribute{
				MarkdownDescription: "Display name of the owning workspace.",
				Computed:            true,
			},
			"workspace_slug": schema.StringAttribute{
				MarkdownDescription: "URL-safe identifier of the owning workspace.",
				Computed:            true,
			},
			"workspace_type": schema.StringAttribute{
				MarkdownDescription: "What the owning workspace represents, e.g. `TEAM`.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of when the account was connected.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of the last change to the account.",
				Computed:            true,
			},
		},
	}
}

func (d *accountDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, resp)
}

func (d *accountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config accountDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	account, err := d.client.Accounts.Get(ctx, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("read the account", err))
		return
	}

	state := accountDataModel{
		ID:            types.StringValue(account.ID),
		WorkspaceID:   optionalString(firstNonEmpty(account.WorkspaceID, account.Workspace.ID)),
		Platform:      optionalString(account.Platform),
		Username:      optionalString(account.Username),
		Name:          optionalString(account.Name),
		Avatar:        optionalString(account.Avatar),
		WorkspaceName: optionalString(account.Workspace.Name),
		WorkspaceSlug: optionalString(account.Workspace.Slug),
		WorkspaceType: optionalString(account.Workspace.Type),
		CreatedAt:     timestamp(account.CreatedAt),
		UpdatedAt:     timestamp(account.UpdatedAt),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
