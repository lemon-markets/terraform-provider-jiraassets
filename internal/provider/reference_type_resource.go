package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/ctreminiom/go-atlassian/v2/assets"
	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
	client       *assets.Client
	workspace_id string
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
		Description: "A Jira Assets reference type resource, used to describe the relationship an object reference attribute represents. Hand-rolled: the config/referencetype endpoint has no go-atlassian connector support.",
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
				Description: "The ID of the reference type. Unlike object types and attributes, this is a UUID, not a small integer.",
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
				Optional: true,
				Description: "The object schema this reference type is scoped to. Omitted, the reference type is global. " +
					"The API silently ignores changes to this on update -- PUT with a different value returns 200 with " +
					"the OLD schema id still in the response body -- so it requires replacement rather than risking a " +
					"schema move that looks applied but is not.",
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

	referenceType, err := r.createReferenceType(ctx, payload)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error during reference type creation",
			err.Error(),
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

	referenceType, err := r.getReferenceType(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error during reference type reading",
			err.Error(),
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

	referenceType, err := r.updateReferenceType(ctx, plan.Id.ValueString(), payload)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error during reference type update",
			err.Error(),
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

	if err := r.deleteReferenceType(ctx, state.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error during reference type deletion",
			err.Error(),
		)
		return
	}
}

func (r *referenceTypeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *referenceTypeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerClient, ok := req.ProviderData.(JiraAssetsProviderClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *assets.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = providerClient.client
	r.workspace_id = providerClient.workspaceId
}

func (r *referenceTypeResource) createReferenceType(ctx context.Context, payload referenceTypePayload) (*referenceTypeScheme, error) {
	endpoint := fmt.Sprintf("jsm/assets/workspace/%v/v1/config/referencetype", r.workspace_id)

	httpReq, err := r.client.NewRequest(ctx, http.MethodPost, endpoint, "", payload)
	if err != nil {
		return nil, err
	}

	referenceType := new(referenceTypeScheme)
	response, err := r.client.Call(httpReq, referenceType)
	if err != nil {
		logReferenceTypeError(ctx, "Error creating reference type", response, err)
		return nil, err
	}

	return referenceType, nil
}

func (r *referenceTypeResource) getReferenceType(ctx context.Context, id string) (*referenceTypeScheme, error) {
	endpoint := fmt.Sprintf("jsm/assets/workspace/%v/v1/config/referencetype/%v", r.workspace_id, id)

	httpReq, err := r.client.NewRequest(ctx, http.MethodGet, endpoint, "", nil)
	if err != nil {
		return nil, err
	}

	referenceType := new(referenceTypeScheme)
	response, err := r.client.Call(httpReq, referenceType)
	if err != nil {
		logReferenceTypeError(ctx, "Error reading reference type", response, err)
		return nil, err
	}

	return referenceType, nil
}

func (r *referenceTypeResource) updateReferenceType(ctx context.Context, id string, payload referenceTypePayload) (*referenceTypeScheme, error) {
	endpoint := fmt.Sprintf("jsm/assets/workspace/%v/v1/config/referencetype/%v", r.workspace_id, id)

	httpReq, err := r.client.NewRequest(ctx, http.MethodPut, endpoint, "", payload)
	if err != nil {
		return nil, err
	}

	referenceType := new(referenceTypeScheme)
	response, err := r.client.Call(httpReq, referenceType)
	if err != nil {
		logReferenceTypeError(ctx, "Error updating reference type", response, err)
		return nil, err
	}

	return referenceType, nil
}

func (r *referenceTypeResource) deleteReferenceType(ctx context.Context, id string) error {
	endpoint := fmt.Sprintf("jsm/assets/workspace/%v/v1/config/referencetype/%v", r.workspace_id, id)

	httpReq, err := r.client.NewRequest(ctx, http.MethodDelete, endpoint, "", nil)
	if err != nil {
		return err
	}

	response, err := r.client.Call(httpReq, nil)
	if err != nil {
		logReferenceTypeError(ctx, "Error deleting reference type", response, err)
		return err
	}

	return nil
}

func logReferenceTypeError(ctx context.Context, message string, response *models.ResponseScheme, err error) {
	if response == nil {
		return
	}

	tflog.Error(ctx, message, map[string]interface{}{
		"url":         response.Request.URL,
		"status_code": response.StatusCode,
		"headers":     response.Header,
		"body":        response.Body,
	})
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
