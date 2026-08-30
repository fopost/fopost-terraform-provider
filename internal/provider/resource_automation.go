package provider

import (
	"context"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*automationResource)(nil)
	_ resource.ResourceWithConfigure   = (*automationResource)(nil)
	_ resource.ResourceWithImportState = (*automationResource)(nil)
)

// NewAutomationResource manages a FoPost automation.
func NewAutomationResource() resource.Resource { return &automationResource{} }

type automationResource struct {
	client *fopost.Client
}

type automationResourceModel struct {
	ID              types.String         `tfsdk:"id"`
	WorkspaceID     types.String         `tfsdk:"workspace_id"`
	Name            types.String         `tfsdk:"name"`
	TriggerType     types.String         `tfsdk:"trigger_type"`
	TriggerConfig   jsontypes.Normalized `tfsdk:"trigger_config"`
	Active          types.Bool           `tfsdk:"active"`
	Steps           types.List           `tfsdk:"step"`
	Secret          types.String         `tfsdk:"secret"`
	RunCount        types.Int64          `tfsdk:"run_count"`
	LastTriggeredAt types.String         `tfsdk:"last_triggered_at"`
	CreatedAt       types.String         `tfsdk:"created_at"`
	UpdatedAt       types.String         `tfsdk:"updated_at"`
}

type automationStepModel struct {
	Position     types.Int64          `tfsdk:"position"`
	ActionType   types.String         `tfsdk:"action_type"`
	ActionConfig jsontypes.Normalized `tfsdk:"action_config"`
}

func automationStepAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"position":      types.Int64Type,
		"action_type":   types.StringType,
		"action_config": jsontypes.NormalizedType{},
	}
}

func (r *automationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automation"
}

func (r *automationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A FoPost automation: one trigger and the ordered steps it runs. The steps " +
			"are declared in order, and the API numbers them from that order.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The automation's server-assigned identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the workspace the automation runs in. An automation " +
					"cannot move between workspaces, so changing this replaces it.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the automation.",
				Required:            true,
			},
			"trigger_type": schema.StringAttribute{
				MarkdownDescription: "What starts a run. One of `cross_post`, `rss_feed`, `api_webhook`, " +
					"or `schedule`. The trigger type is fixed once created, so changing it replaces the " +
					"automation.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"trigger_config": schema.StringAttribute{
				MarkdownDescription: "Trigger settings as a JSON object, e.g. " +
					"`jsonencode({ feed_url = \"https://example.com/feed.xml\" })`. The shape depends on " +
					"`trigger_type`. Compared semantically, so formatting never produces a diff.",
				Optional:   true,
				Computed:   true,
				CustomType: jsontypes.NormalizedType{},
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the automation runs when its trigger fires. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "Signing secret for an `api_webhook` trigger. The API returns it only " +
					"at creation, so an imported automation has none and the value stays null.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"run_count": schema.Int64Attribute{
				MarkdownDescription: "How many times the automation has run.",
				Computed:            true,
			},
			"last_triggered_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of the last run.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of when the automation was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of the last change to the automation.",
				Computed:            true,
			},
		},
		Blocks: map[string]schema.Block{
			"step": schema.ListNestedBlock{
				MarkdownDescription: "One action in the automation. Declare at least one; the order of " +
					"the blocks is the order the steps run in.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"position": schema.Int64Attribute{
							MarkdownDescription: "The step's server-assigned position, taken from the " +
								"order the blocks are declared in.",
							Computed: true,
						},
						"action_type": schema.StringAttribute{
							MarkdownDescription: "What the step does. One of `publish`, `delay`, or " +
								"`transform`.",
							Required: true,
						},
						"action_config": schema.StringAttribute{
							MarkdownDescription: "Step settings as a JSON object, e.g. " +
								"`jsonencode({ minutes = 30 })` for a `delay`. The shape depends on " +
								"`action_type`. Compared semantically, so formatting never produces a diff.",
							Optional:   true,
							Computed:   true,
							CustomType: jsontypes.NormalizedType{},
						},
					},
				},
				Validators: []validator.List{listvalidator.SizeAtLeast(1)},
			},
		},
	}
}

func (r *automationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, resp)
}

func (r *automationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan automationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	steps, diags := automationStepsFromPlan(ctx, plan.Steps)
	resp.Diagnostics.Append(diags...)
	triggerConfig, err := decodeJSON(plan.TriggerConfig)
	if err != nil {
		resp.Diagnostics.AddError("Invalid trigger_config", "trigger_config must be a JSON object: "+err.Error())
	}
	if resp.Diagnostics.HasError() {
		return
	}

	automation, err := r.client.Automations.Create(ctx, &fopost.CreateAutomationRequest{
		WorkspaceID:   str(plan.WorkspaceID),
		Name:          str(plan.Name),
		TriggerType:   str(plan.TriggerType),
		TriggerConfig: triggerConfig,
		Steps:         steps,
		Active:        boolPtr(plan.Active),
	})
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("create the automation", err))
		return
	}

	// The secret arrives once, with the create response.
	plan.Secret = optionalString(automation.Secret)
	applyAutomation(ctx, &plan, automation, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *automationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state automationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	automation, err := r.client.Automations.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(apiDiagnostic("read the automation", err))
		return
	}

	applyAutomation(ctx, &state, automation, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *automationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan automationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state automationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	steps, diags := automationStepsFromPlan(ctx, plan.Steps)
	resp.Diagnostics.Append(diags...)
	triggerConfig, err := decodeJSON(plan.TriggerConfig)
	if err != nil {
		resp.Diagnostics.AddError("Invalid trigger_config", "trigger_config must be a JSON object: "+err.Error())
	}
	if resp.Diagnostics.HasError() {
		return
	}

	automation, err := r.client.Automations.Update(ctx, state.ID.ValueString(), &fopost.UpdateAutomationRequest{
		Name:          str(plan.Name),
		TriggerConfig: triggerConfig,
		Steps:         steps,
		Active:        boolPtr(plan.Active),
	})
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("update the automation", err))
		return
	}

	plan.ID = state.ID
	// The secret is shown once at creation and never again.
	plan.Secret = state.Secret
	applyAutomation(ctx, &plan, automation, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *automationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state automationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Automations.Delete(ctx, state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.Append(apiDiagnostic("delete the automation", err))
	}
}

func (r *automationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, importIDPath, req, resp)
}

func automationStepsFromPlan(ctx context.Context, list types.List) ([]fopost.AutomationStep, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var models []automationStepModel
	diags.Append(list.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil, diags
	}

	steps := make([]fopost.AutomationStep, 0, len(models))
	for i, stepModel := range models {
		config, err := decodeJSON(stepModel.ActionConfig)
		if err != nil {
			diags.AddError(
				"Invalid action_config",
				"Step "+str(stepModel.ActionType)+" has an action_config that is not a JSON object: "+err.Error(),
			)
			return nil, diags
		}
		steps = append(steps, fopost.AutomationStep{
			Position:     i + 1,
			ActionType:   str(stepModel.ActionType),
			ActionConfig: config,
		})
	}
	return steps, diags
}

func applyAutomation(ctx context.Context, model *automationResourceModel, automation *fopost.Automation, diags *diag.Diagnostics) {
	if automation.ID != "" {
		model.ID = types.StringValue(automation.ID)
	}
	if automation.WorkspaceID != "" {
		model.WorkspaceID = types.StringValue(automation.WorkspaceID)
	}
	model.Name = types.StringValue(automation.Name)
	model.TriggerType = types.StringValue(automation.TriggerType)
	model.Active = types.BoolValue(automation.Active)
	model.RunCount = types.Int64Value(int64(automation.RunCount))
	model.LastTriggeredAt = timestamp(automation.LastTriggeredAt)
	model.CreatedAt = timestamp(automation.CreatedAt)
	model.UpdatedAt = timestamp(automation.UpdatedAt)

	triggerConfig, err := encodeJSON(automation.TriggerConfig)
	if err != nil {
		diags.AddError("Could Not Read trigger_config", "The API returned a trigger_config that will not encode: "+err.Error())
		return
	}
	model.TriggerConfig = triggerConfig

	stepModels := make([]automationStepModel, 0, len(automation.Steps))
	for _, step := range automation.Steps {
		actionConfig, err := encodeJSON(step.ActionConfig)
		if err != nil {
			diags.AddError("Could Not Read action_config", "The API returned an action_config that will not encode: "+err.Error())
			return
		}
		stepModels = append(stepModels, automationStepModel{
			Position:     types.Int64Value(int64(step.Position)),
			ActionType:   types.StringValue(step.ActionType),
			ActionConfig: actionConfig,
		})
	}
	steps, stepDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: automationStepAttrTypes()}, stepModels)
	diags.Append(stepDiags...)
	if !stepDiags.HasError() {
		model.Steps = steps
	}
}
