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
	_ datasource.DataSource              = (*labelsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*labelsDataSource)(nil)
)

// NewLabelsDataSource lists labels.
func NewLabelsDataSource() datasource.DataSource { return &labelsDataSource{} }

type labelsDataSource struct {
	client *fopost.Client
}

type labelsDataModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Labels      types.List   `tfsdk:"labels"`
}

type labelListItemModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Name        types.String `tfsdk:"name"`
	Color       types.String `tfsdk:"color"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func labelListItemAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":           types.StringType,
		"workspace_id": types.StringType,
		"name":         types.StringType,
		"color":        types.StringType,
		"created_at":   types.StringType,
		"updated_at":   types.StringType,
	}
}

func (d *labelsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_labels"
}

func (d *labelsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the FoPost labels the API key can reach. Use it to reference labels " +
			"created outside this configuration.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Narrow the list to one workspace. Omit it to list every label the " +
					"API key can reach.",
				Optional: true,
			},
			"labels": schema.ListNestedAttribute{
				MarkdownDescription: "The labels, in the order the API returned them.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The label's identifier.",
							Computed:            true,
						},
						"workspace_id": schema.StringAttribute{
							MarkdownDescription: "Identifier of the workspace the label belongs to.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Display name of the label.",
							Computed:            true,
						},
						"color": schema.StringAttribute{
							MarkdownDescription: "Hex color the label is drawn in.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "RFC 3339 timestamp of when the label was created.",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "RFC 3339 timestamp of the last change to the label.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *labelsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, resp)
}

func (d *labelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config labelsDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	labels, err := d.client.Labels.List(ctx, str(config.WorkspaceID))
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("list the labels", err))
		return
	}

	items := make([]labelListItemModel, 0, len(labels))
	for _, label := range labels {
		item := labelListItemModel{
			ID:        types.StringValue(label.ID),
			Name:      optionalString(label.Name),
			Color:     optionalString(label.Color),
			CreatedAt: timestamp(label.CreatedAt),
			UpdatedAt: timestamp(label.UpdatedAt),
		}
		if label.Workspace != nil {
			item.WorkspaceID = optionalString(label.Workspace.ID)
		} else {
			item.WorkspaceID = types.StringNull()
		}
		items = append(items, item)
	}

	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: labelListItemAttrTypes()}, items)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Labels = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
