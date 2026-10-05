package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// hexColorRegexp matches the hex color strings the reference type color field accepts.
var hexColorRegexp = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &referenceTypeResource{}
	_ resource.ResourceWithConfigure   = &referenceTypeResource{}
	_ resource.ResourceWithImportState = &referenceTypeResource{}
)

// referenceTypePayload and referenceTypeScheme model the config/referencetype
// endpoint, which go-atlassian does not support; requests are built and sent
// directly with client.NewRequest/client.Call.
type referenceTypePayload struct {
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	Color          string `json:"color,omitempty"`
	ObjectSchemaID string `json:"objectSchemaId,omitempty"`
}

type referenceTypeScheme struct {
	WorkspaceID    string `json:"workspaceId,omitempty"`
	GlobalID       string `json:"globalId,omitempty"`
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	Color          string `json:"color,omitempty"`
	ObjectSchemaID string `json:"objectSchemaId,omitempty"`
	Removable      bool   `json:"removable,omitempty"`
}

// NewReferenceTypeResource is a helper function to simplify the provider implementation.
func NewReferenceTypeResource() resource.Resource {
	return &referenceTypeResource{}
}

// referenceTypeResource is the resource implementation.
type referenceTypeResource struct {
	apiClient
}

func (r *referenceTypeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reference_type"
}

type referenceTypeResourceModel struct {
	WorkspaceId    types.String `tfsdk:"workspace_id"`
	GlobalId       types.String `tfsdk:"global_id"`
	Id             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Color          types.String `tfsdk:"color"`
	ObjectSchemaId types.String `tfsdk:"object_schema_id"`
	Removable      types.Bool   `tfsdk:"removable"`
}

// Schema defines the schema for the resource.
func (r *referenceTypeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Jira Assets reference type, naming the relationship an object reference attribute represents.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"global_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the reference type.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the reference type. Must be unique within the object schema.",
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"color": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "A hex color string, e.g. 42526E.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(hexColorRegexp, "must be a hex color string, e.g. 42526E"),
				},
			},
			"object_schema_id": schema.StringAttribute{
				Optional:    true,
				Description: "The object schema this reference type is scoped to; global if omitted. Forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"removable": schema.BoolAttribute{
				Computed: true,
			},
		},
	}
}

func (r *referenceTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan referenceTypeResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := referenceTypePayload{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		Color:          plan.Color.ValueString(),
		ObjectSchemaID: plan.ObjectSchemaId.ValueString(),
	}

	referenceType := new(referenceTypeScheme)
	response, err := r.call(ctx, http.MethodPost, "", payload, referenceType)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error during reference type creation",
			apiError(err, response),
		)
		return
	}

	plan = referenceTypeModelFromScheme(referenceType)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *referenceTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state referenceTypeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	referenceType := new(referenceTypeScheme)
	response, err := r.call(ctx, http.MethodGet, state.Id.ValueString(), nil, referenceType)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error during reference type reading",
			apiError(err, response),
		)
		return
	}

	state = referenceTypeModelFromScheme(referenceType)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *referenceTypeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan referenceTypeResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := referenceTypePayload{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Color:       plan.Color.ValueString(),
	}

	referenceType := new(referenceTypeScheme)
	response, err := r.call(ctx, http.MethodPut, plan.Id.ValueString(), payload, referenceType)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error during reference type update",
			apiError(err, response),
		)
		return
	}

	plan = referenceTypeModelFromScheme(referenceType)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *referenceTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state referenceTypeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if response, err := r.call(ctx, http.MethodDelete, state.Id.ValueString(), nil, nil); err != nil {
		resp.Diagnostics.AddError(
			"Error during reference type deletion",
			apiError(err, response),
		)
		return
	}
}

func (r *referenceTypeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *referenceTypeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req.ProviderData, &resp.Diagnostics)
}

// call sends one request to config/referencetype, or to config/referencetype/{id}
// when id is set, decoding the response into out unless it is nil.
func (r *referenceTypeResource) call(ctx context.Context, method, id string, payload any, out any) (*models.ResponseScheme, error) {
	endpoint := fmt.Sprintf("jsm/assets/workspace/%v/v1/config/referencetype", r.workspace_id)
	if id != "" {
		endpoint += "/" + id
	}

	httpReq, err := r.client.NewRequest(ctx, method, endpoint, "", payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.Call(httpReq, out)
	if err != nil {
		logAPIError(ctx, fmt.Sprintf("Error calling %s %s", method, endpoint), response)
	}

	return response, err
}

func referenceTypeModelFromScheme(s *referenceTypeScheme) referenceTypeResourceModel {
	return referenceTypeResourceModel{
		WorkspaceId:    types.StringValue(s.WorkspaceID),
		GlobalId:       types.StringValue(s.GlobalID),
		Id:             types.StringValue(s.ID),
		Name:           types.StringValue(s.Name),
		Description:    types.StringValue(s.Description),
		Color:          types.StringValue(s.Color),
		ObjectSchemaId: optionalStringValue(s.ObjectSchemaID),
		Removable:      types.BoolValue(s.Removable),
	}
}
