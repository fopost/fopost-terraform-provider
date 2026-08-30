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
	_ datasource.DataSource              = (*workspacesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*workspacesDataSource)(nil)
)

// NewWorkspacesDataSource lists every workspace the API key can reach.
func NewWorkspacesDataSource() datasource.DataSource { return &workspacesDataSource{} }

type workspacesDataSource struct {
	client *fopost.Client
}

type workspacesDataModel struct {
	Workspaces types.List `tfsdk:"workspaces"`
}

func workspaceObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":          types.StringType,
		"name":        types.StringType,
		"slug":        types.StringType,
		"type":        types.StringType,
		"logo":        types.StringType,
		"website":     types.StringType,
		"timezone":    types.StringType,
		"country":     types.StringType,
		"description": types.StringType,
		"language":    types.StringType,
		"accounts":    types.ListType{ElemType: types.ObjectType{AttrTypes: workspaceAccountAttrTypes()}},
		"created_at":  types.StringType,
		"updated_at":  types.StringType,
	}
}

func (d *workspacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspaces"
}

func (d *workspacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every FoPost workspace the API key can reach. A key bound to a single " +
			"workspace sees only that one.",
		Attributes: map[string]schema.Attribute{
			"workspaces": schema.ListNestedAttribute{
				MarkdownDescription: "The workspaces, in the order the API returned them.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: workspaceAttributes("The workspace's identifier.", false),
				},
			},
		},
	}
}

func (d *workspacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, resp)
}

func (d *workspacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	workspaces, err := d.client.Workspaces.List(ctx)
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("list the workspaces", err))
		return
	}

	models := make([]workspaceDataModel, 0, len(workspaces))
	for i := range workspaces {
		models = append(models, workspaceDataValue(ctx, &workspaces[i], &resp.Diagnostics))
	}
	if resp.Diagnostics.HasError() {
		return
	}

	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: workspaceObjectAttrTypes()}, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &workspacesDataModel{Workspaces: list})...)
}
