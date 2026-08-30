package provider

import (
	"context"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*labelResource)(nil)
	_ resource.ResourceWithConfigure   = (*labelResource)(nil)
	_ resource.ResourceWithImportState = (*labelResource)(nil)
)

// NewLabelResource manages a FoPost label.
func NewLabelResource() resource.Resource { return &labelResource{} }

type labelResource struct {
	client *fopost.Client
}

type labelResourceModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Name        types.String `tfsdk:"name"`
	Color       types.String `tfsdk:"color"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *labelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (r *labelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A FoPost label: the campaign tag posts are grouped and reported by. " +
			"Deleting a label unlinks it from every post carrying it; the posts themselves are untouched.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The label's server-assigned identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the workspace the label belongs to. A label cannot " +
					"move between workspaces, so changing this replaces the label.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the label.",
				Required:            true,
			},
			"color": schema.StringAttribute{
				MarkdownDescription: "Hex color the label is drawn in, e.g. `#2563eb`.",
				Required:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of when the label was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of the last change to the label.",
				Computed:            true,
			},
		},
	}
}

func (r *labelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, resp)
}

func (r *labelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan labelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label, err := r.client.Labels.Create(ctx, &fopost.CreateLabelRequest{
		WorkspaceID: str(plan.WorkspaceID),
		Name:        str(plan.Name),
		Color:       str(plan.Color),
	})
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("create the label", err))
		return
	}

	applyLabel(&plan, label)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *labelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state labelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label, err := r.client.Labels.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(apiDiagnostic("read the label", err))
		return
	}

	applyLabel(&state, label)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *labelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan labelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state labelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label, err := r.client.Labels.Update(ctx, state.ID.ValueString(), &fopost.UpdateLabelRequest{
		Name:  str(plan.Name),
		Color: str(plan.Color),
	})
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("update the label", err))
		return
	}

	plan.ID = state.ID
	applyLabel(&plan, label)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *labelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state labelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Labels.Delete(ctx, state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.Append(apiDiagnostic("delete the label", err))
	}
}

func (r *labelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, importIDPath, req, resp)
}

func applyLabel(model *labelResourceModel, label *fopost.Label) {
	if label.ID != "" {
		model.ID = types.StringValue(label.ID)
	}
	model.Name = types.StringValue(label.Name)
	model.Color = types.StringValue(label.Color)
	// The API nests the owning workspace rather than echoing the id, and omits
	// it on some responses — an omission is not a move, so state is kept.
	if label.Workspace != nil && label.Workspace.ID != "" {
		model.WorkspaceID = types.StringValue(label.Workspace.ID)
	}
	model.CreatedAt = timestamp(label.CreatedAt)
	model.UpdatedAt = timestamp(label.UpdatedAt)
}
