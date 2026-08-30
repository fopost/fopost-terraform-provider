package provider

import (
	"context"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*workspaceDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*workspaceDataSource)(nil)
)

// NewWorkspaceDataSource reads one workspace by id.
func NewWorkspaceDataSource() datasource.DataSource { return &workspaceDataSource{} }

type workspaceDataSource struct {
	client *fopost.Client
}

type workspaceDataModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Type        types.String `tfsdk:"type"`
	Logo        types.String `tfsdk:"logo"`
	Website     types.String `tfsdk:"website"`
	Timezone    types.String `tfsdk:"timezone"`
	Country     types.String `tfsdk:"country"`
	Description types.String `tfsdk:"description"`
	Language    types.String `tfsdk:"language"`
	Accounts    types.List   `tfsdk:"accounts"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type workspaceAccountModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Platform    types.String `tfsdk:"platform"`
	Username    types.String `tfsdk:"username"`
	Name        types.String `tfsdk:"name"`
	Avatar      types.String `tfsdk:"avatar"`
}

func workspaceAccountAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":           types.StringType,
		"workspace_id": types.StringType,
		"platform":     types.StringType,
		"username":     types.StringType,
		"name":         types.StringType,
		"avatar":       types.StringType,
	}
}

func workspaceAccountsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		MarkdownDescription: "Social accounts connected to the workspace.",
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
			},
		},
	}
}

// workspaceAttributes is the read-only shape shared by the single and plural
// workspace data sources.
func workspaceAttributes(idDescription string, idRequired bool) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: idDescription,
			Required:            idRequired,
			Computed:            !idRequired,
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Display name of the workspace.",
			Computed:            true,
		},
		"slug": schema.StringAttribute{
			MarkdownDescription: "URL-safe identifier of the workspace.",
			Computed:            true,
		},
		"type": schema.StringAttribute{
			MarkdownDescription: "What the workspace represents, e.g. `TEAM` or `AGENCY`.",
			Computed:            true,
		},
		"logo": schema.StringAttribute{
			MarkdownDescription: "URL of the workspace logo.",
			Computed:            true,
		},
		"website": schema.StringAttribute{
			MarkdownDescription: "The brand's website.",
			Computed:            true,
		},
		"timezone": schema.StringAttribute{
			MarkdownDescription: "IANA time zone scheduled posts are read in.",
			Computed:            true,
		},
		"country": schema.StringAttribute{
			MarkdownDescription: "ISO 3166-1 alpha-2 country code.",
			Computed:            true,
		},
		"description": schema.StringAttribute{
			MarkdownDescription: "Free-text description of the workspace.",
			Computed:            true,
		},
		"language": schema.StringAttribute{
			MarkdownDescription: "Primary content language as an ISO 639-1 code.",
			Computed:            true,
		},
		"accounts": workspaceAccountsSchema(),
		"created_at": schema.StringAttribute{
			MarkdownDescription: "RFC 3339 timestamp of when the workspace was created.",
			Computed:            true,
		},
		"updated_at": schema.StringAttribute{
			MarkdownDescription: "RFC 3339 timestamp of the last change to the workspace.",
			Computed:            true,
		},
	}
}

func (d *workspaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *workspaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one FoPost workspace and the social accounts connected to it. Use it " +
			"to reference a workspace this configuration does not manage.",
		Attributes: workspaceAttributes("Identifier of the workspace to read.", true),
	}
}

func (d *workspaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, resp)
}

func (d *workspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config workspaceDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspace, err := d.client.Workspaces.Get(ctx, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("read the workspace", err))
		return
	}

	state := workspaceDataValue(ctx, workspace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func workspaceDataValue(ctx context.Context, workspace *fopost.Workspace, diags *diag.Diagnostics) workspaceDataModel {
	accounts := make([]workspaceAccountModel, 0, len(workspace.Accounts))
	for _, account := range workspace.Accounts {
		accounts = append(accounts, workspaceAccountModel{
			ID:          types.StringValue(account.ID),
			WorkspaceID: optionalString(account.WorkspaceID),
			Platform:    optionalString(account.Platform),
			Username:    optionalString(account.Username),
			Name:        optionalString(account.Name),
			Avatar:      optionalString(account.Avatar),
		})
	}
	accountList, accountDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: workspaceAccountAttrTypes()}, accounts)
	diags.Append(accountDiags...)

	return workspaceDataModel{
		ID:          types.StringValue(workspace.ID),
		Name:        optionalString(workspace.Name),
		Slug:        optionalString(workspace.Slug),
		Type:        optionalString(workspace.Type),
		Logo:        optionalString(workspace.Logo),
		Website:     optionalString(workspace.Website),
		Timezone:    optionalString(workspace.Timezone),
		Country:     optionalString(workspace.Country),
		Description: optionalString(workspace.Description),
		Language:    optionalString(workspace.Language),
		Accounts:    accountList,
		CreatedAt:   timestamp(workspace.CreatedAt),
		UpdatedAt:   timestamp(workspace.UpdatedAt),
	}
}
