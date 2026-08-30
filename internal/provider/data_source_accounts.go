package provider

import (
	"context"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*accountsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*accountsDataSource)(nil)
)

// NewAccountsDataSource lists connected social accounts.
func NewAccountsDataSource() datasource.DataSource { return &accountsDataSource{} }

type accountsDataSource struct {
	client *fopost.Client
}

type accountsDataModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Accounts    types.List   `tfsdk:"accounts"`
}

type accountListItemModel struct {
	ID              types.String `tfsdk:"id"`
	WorkspaceID     types.String `tfsdk:"workspace_id"`
	Platform        types.String `tfsdk:"platform"`
	Username        types.String `tfsdk:"username"`
	Name            types.String `tfsdk:"name"`
	Avatar          types.String `tfsdk:"avatar"`
	IsPrimary       types.Bool   `tfsdk:"is_primary"`
	Active          types.Bool   `tfsdk:"active"`
	HealthStatus    types.String `tfsdk:"health_status"`
	LastHealthCheck types.String `tfsdk:"last_health_check"`
}

func accountListItemAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                types.StringType,
		"workspace_id":      types.StringType,
		"platform":          types.StringType,
		"username":          types.StringType,
		"name":              types.StringType,
		"avatar":            types.StringType,
		"is_primary":        types.BoolType,
		"active":            types.BoolType,
		"health_status":     types.StringType,
		"last_health_check": types.StringType,
	}
}

func (d *accountsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_accounts"
}

func (d *accountsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the connected social accounts the API key can reach, with the " +
			"connection health FoPost last recorded for each.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Narrow the list to one workspace. Omit it to list every account the " +
					"API key can reach.",
				Optional: true,
			},
			"accounts": schema.ListNestedAttribute{
				MarkdownDescription: "The connected accounts, in the order the API returned them.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The account's identifier.",
							Computed:            true,
						},
						"workspace_id": schema.StringAttribute{
							MarkdownDescription: "Identifier of the workspace the account belongs to.",
							Computed:            true,
						},
						"platform": schema.StringAttribute{
							MarkdownDescription: "The social network the account is on.",
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
						"is_primary": schema.BoolAttribute{
							MarkdownDescription: "Whether the account leads its platform in the workspace.",
							Computed:            true,
						},
						"active": schema.BoolAttribute{
							MarkdownDescription: "Whether FoPost will publish to the account.",
							Computed:            true,
						},
						"health_status": schema.StringAttribute{
							MarkdownDescription: "Connection health: `healthy`, `degraded`, `expired`, " +
								"`revoked`, or `unknown`.",
							Computed: true,
						},
						"last_health_check": schema.StringAttribute{
							MarkdownDescription: "RFC 3339 timestamp of the last health check.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *accountsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, resp)
}

func (d *accountsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config accountsDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accounts, err := d.client.Accounts.List(ctx, str(config.WorkspaceID))
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("list the connected accounts", err))
		return
	}

	items := make([]accountListItemModel, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, accountListItemModel{
			ID:              types.StringValue(account.ID),
			WorkspaceID:     optionalString(account.WorkspaceID),
			Platform:        optionalString(account.Platform),
			Username:        optionalString(account.Username),
			Name:            optionalString(account.Name),
			Avatar:          optionalString(account.Avatar),
			IsPrimary:       types.BoolValue(account.IsPrimary),
			Active:          types.BoolValue(account.Active),
			HealthStatus:    optionalString(account.HealthStatus),
			LastHealthCheck: timestamp(account.LastHealthCheck),
		})
	}

	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: accountListItemAttrTypes()}, items)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Accounts = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
