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
	_ resource.Resource                = (*workspaceResource)(nil)
	_ resource.ResourceWithConfigure   = (*workspaceResource)(nil)
	_ resource.ResourceWithImportState = (*workspaceResource)(nil)
)

// NewWorkspaceResource manages a FoPost workspace.
func NewWorkspaceResource() resource.Resource { return &workspaceResource{} }

type workspaceResource struct {
	client *fopost.Client
}

type workspaceResourceModel struct {
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
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *workspaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (r *workspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A FoPost workspace: the tenant boundary every connected account, label, " +
			"webhook, and automation is scoped to. Plans cap how many workspaces an account may hold, so " +
			"creating one past the limit fails with a subscription error.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The workspace's server-assigned identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the workspace.",
				Required:            true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "URL-safe identifier, unique across the account.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "What the workspace represents. One of `PERSONAL`, `TEAM`, " +
					"`ORGANIZATION`, `CLIENT`, `PROJECT`, `DEPARTMENT`, `EVENT`, `TEMPORARY`, `COMMUNITY`, " +
					"`BRAND`, or `AGENCY`. The API picks a default when this is omitted.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"logo": schema.StringAttribute{
				MarkdownDescription: "URL of the workspace logo.",
				Optional:            true,
			},
			"website": schema.StringAttribute{
				MarkdownDescription: "The brand's website.",
				Optional:            true,
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "IANA time zone that scheduled posts in this workspace are read in, " +
					"e.g. `Europe/Berlin`. The API picks a default when this is omitted.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"country": schema.StringAttribute{
				MarkdownDescription: "ISO 3166-1 alpha-2 country code, e.g. `DE`.",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-text description of the workspace.",
				Optional:            true,
			},
			"language": schema.StringAttribute{
				MarkdownDescription: "Primary content language as an ISO 639-1 code, e.g. `en`. The API " +
					"picks a default when this is omitted.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of when the workspace was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of the last change to the workspace.",
				Computed:            true,
			},
		},
	}
}

func (r *workspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, resp)
}

func (r *workspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspace, err := r.client.Workspaces.Create(ctx, &fopost.CreateWorkspaceRequest{
		Name:        str(plan.Name),
		Slug:        str(plan.Slug),
		Type:        str(plan.Type),
		Logo:        optionalStrPtr(plan.Logo),
		Website:     optionalStrPtr(plan.Website),
		Timezone:    str(plan.Timezone),
		Country:     optionalStrPtr(plan.Country),
		Description: optionalStrPtr(plan.Description),
		Language:    str(plan.Language),
	})
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("create the workspace", err))
		return
	}

	applyWorkspace(&plan, workspace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspace, err := r.client.Workspaces.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(apiDiagnostic("read the workspace", err))
		return
	}

	applyWorkspace(&state, workspace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state workspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The optional strings are sent even when empty, so removing one from the
	// configuration clears it upstream instead of drifting forever.
	workspace, err := r.client.Workspaces.Update(ctx, state.ID.ValueString(), &fopost.UpdateWorkspaceRequest{
		Name:        str(plan.Name),
		Slug:        str(plan.Slug),
		Type:        str(plan.Type),
		Logo:        strPtr(plan.Logo),
		Website:     strPtr(plan.Website),
		Timezone:    str(plan.Timezone),
		Country:     strPtr(plan.Country),
		Description: strPtr(plan.Description),
		Language:    str(plan.Language),
	})
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("update the workspace", err))
		return
	}

	plan.ID = state.ID
	applyWorkspace(&plan, workspace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Workspaces.Delete(ctx, state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.Append(apiDiagnostic("delete the workspace", err))
	}
}

func (r *workspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, importIDPath, req, resp)
}

func applyWorkspace(model *workspaceResourceModel, workspace *fopost.Workspace) {
	if workspace.ID != "" {
		model.ID = types.StringValue(workspace.ID)
	}
	model.Name = types.StringValue(workspace.Name)
	model.Slug = types.StringValue(workspace.Slug)
	model.Type = optionalString(workspace.Type)
	model.Logo = optionalString(workspace.Logo)
	model.Website = optionalString(workspace.Website)
	model.Timezone = optionalString(workspace.Timezone)
	model.Country = optionalString(workspace.Country)
	model.Description = optionalString(workspace.Description)
	model.Language = optionalString(workspace.Language)
	model.CreatedAt = timestamp(workspace.CreatedAt)
	model.UpdatedAt = timestamp(workspace.UpdatedAt)
}
