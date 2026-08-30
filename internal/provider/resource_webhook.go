package provider

import (
	"context"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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
	_ resource.Resource                = (*webhookResource)(nil)
	_ resource.ResourceWithConfigure   = (*webhookResource)(nil)
	_ resource.ResourceWithImportState = (*webhookResource)(nil)
)

// NewWebhookResource manages a FoPost webhook subscription.
func NewWebhookResource() resource.Resource { return &webhookResource{} }

type webhookResource struct {
	client *fopost.Client
}

type webhookResourceModel struct {
	ID              types.String `tfsdk:"id"`
	WorkspaceID     types.String `tfsdk:"workspace_id"`
	URL             types.String `tfsdk:"url"`
	Events          types.Set    `tfsdk:"events"`
	Active          types.Bool   `tfsdk:"active"`
	Secret          types.String `tfsdk:"secret"`
	FailureCount    types.Int64  `tfsdk:"failure_count"`
	LastTriggeredAt types.String `tfsdk:"last_triggered_at"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A FoPost webhook subscription: the push counterpart to polling a post's " +
			"deliveries. FoPost signs every delivery with the subscription's `secret`, which the API " +
			"returns exactly once, at creation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The subscription's server-assigned identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the workspace whose events are delivered. A " +
					"subscription cannot move between workspaces, so changing this replaces it.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "HTTPS endpoint FoPost posts each event to.",
				Required:            true,
			},
			"events": schema.SetAttribute{
				MarkdownDescription: "Events to subscribe to. One or more of `post.published`, " +
					"`post.failed`, `post.partially_failed`, `delivery.published`, `delivery.failed`, " +
					"`delivery.delayed`, `account.health_changed`.",
				Required:    true,
				ElementType: types.StringType,
				Validators:  []validator.Set{setvalidator.SizeAtLeast(1)},
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether FoPost delivers to this endpoint. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "Signing secret for verifying deliveries. The API returns it only at " +
					"creation, so an imported subscription has none and the value stays null.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"failure_count": schema.Int64Attribute{
				MarkdownDescription: "Consecutive delivery failures recorded against the endpoint.",
				Computed:            true,
			},
			"last_triggered_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of the last delivery attempt.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp of when the subscription was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *webhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, resp)
}

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	events, diags := stringSet(ctx, plan.Events)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.Webhooks.Create(ctx, &fopost.CreateWebhookRequest{
		WorkspaceID: str(plan.WorkspaceID),
		URL:         str(plan.URL),
		Events:      events,
	})
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("create the webhook", err))
		return
	}

	plan.ID = types.StringValue(created.ID)
	plan.Secret = optionalString(created.Secret)
	plan.CreatedAt = timestamp(created.CreatedAt)
	plan.FailureCount = types.Int64Value(0)
	plan.LastTriggeredAt = types.StringNull()
	if created.WorkspaceID != "" {
		plan.WorkspaceID = types.StringValue(created.WorkspaceID)
	}
	plan.URL = types.StringValue(created.URL)

	// The API only honours `active: false` on update, so a subscription asked to
	// start switched off is created and then switched off.
	if !plan.Active.ValueBool() {
		updated, err := r.client.Webhooks.Update(ctx, created.ID, &fopost.UpdateWebhookRequest{
			Active: fopost.Bool(false),
		})
		if err != nil {
			resp.Diagnostics.Append(apiDiagnostic("deactivate the new webhook", err))
			return
		}
		applyWebhook(ctx, &plan, updated, &resp.Diagnostics)
	} else {
		plan.Active = types.BoolValue(created.Active)
		eventSet, diags := eventsValue(ctx, created.Events)
		resp.Diagnostics.Append(diags...)
		plan.Events = eventSet
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API lists subscriptions rather than exposing a per-id read, so a
	// subscription missing from the list is the 404 every other Read handles.
	webhooks, err := r.client.Webhooks.List(ctx)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(apiDiagnostic("read the webhook", err))
		return
	}

	id := state.ID.ValueString()
	for i := range webhooks {
		if webhooks[i].ID != id {
			continue
		}
		applyWebhook(ctx, &state, &webhooks[i], &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan webhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state webhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	events, diags := stringSet(ctx, plan.Events)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.Webhooks.Update(ctx, state.ID.ValueString(), &fopost.UpdateWebhookRequest{
		URL:    str(plan.URL),
		Events: events,
		Active: boolPtr(plan.Active),
	})
	if err != nil {
		resp.Diagnostics.Append(apiDiagnostic("update the webhook", err))
		return
	}

	plan.ID = state.ID
	// The secret is shown once at creation and never again.
	plan.Secret = state.Secret
	applyWebhook(ctx, &plan, updated, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Webhooks.Delete(ctx, state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.Append(apiDiagnostic("delete the webhook", err))
	}
}

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, importIDPath, req, resp)
}

func applyWebhook(ctx context.Context, model *webhookResourceModel, webhook *fopost.Webhook, diags *diag.Diagnostics) {
	if webhook.ID != "" {
		model.ID = types.StringValue(webhook.ID)
	}
	if webhook.WorkspaceID != "" {
		model.WorkspaceID = types.StringValue(webhook.WorkspaceID)
	}
	model.URL = types.StringValue(webhook.URL)
	model.Active = types.BoolValue(webhook.Active)
	model.FailureCount = types.Int64Value(int64(webhook.FailureCount))
	model.LastTriggeredAt = timestamp(webhook.LastTriggeredAt)
	model.CreatedAt = timestamp(webhook.CreatedAt)

	events, eventDiags := eventsValue(ctx, webhook.Events)
	diags.Append(eventDiags...)
	if !eventDiags.HasError() {
		model.Events = events
	}
}
