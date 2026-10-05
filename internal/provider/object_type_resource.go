package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &objectTypeResource{}
	_ resource.ResourceWithConfigure   = &objectTypeResource{}
	_ resource.ResourceWithImportState = &objectTypeResource{}
	_ resource.ResourceWithModifyPlan  = &objectTypeResource{}
)

var (
	schemaPathAttributeIds = path.Root("attribute_ids")
	schemaPathInherited    = path.Root("inherited")
)

// NewObjectTypeResource is a helper function to simplify the provider implementation.
func NewObjectTypeResource() resource.Resource {
	return &objectTypeResource{}
}

// objectTypeResource is the resource implementation.
type objectTypeResource struct {
	apiClient
}

func (r *objectTypeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_type"
}

type objectTypeResourceModel struct {
	WorkspaceId        types.String `tfsdk:"workspace_id"`
	GlobalId           types.String `tfsdk:"global_id"`
	Id                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	IconId             types.String `tfsdk:"icon_id"`
	ObjectSchemaId     types.String `tfsdk:"object_schema_id"`
	ParentObjectTypeId types.String `tfsdk:"parent_object_type_id"`
	AbstractObjectType types.Bool   `tfsdk:"abstract"`
	Inherited          types.Bool   `tfsdk:"inherited"`
	ParentInherited    types.Bool   `tfsdk:"parent_object_type_inherited"`
	Position           types.Int64  `tfsdk:"position"`
	Created            types.String `tfsdk:"created"`
	Updated            types.String `tfsdk:"updated"`
	ObjectCount        types.Int64  `tfsdk:"object_count"`
	Attributes         types.Map    `tfsdk:"attributes"`
	AttributeIds       types.Map    `tfsdk:"attribute_ids"`
}

// Schema defines the schema for the resource.
func (r *objectTypeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Jira Assets object type resource. Deleting an object type that still has objects cascades: the objects become unreadable.",
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
				Description: "The ID of the object type.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the object type.",
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"icon_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the icon of the object type. Required by the API at creation time.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"object_schema_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the object schema this object type belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent_object_type_id": schema.StringAttribute{
				Optional:    true,
				Description: "The ID of the parent object type. An object type has at most one parent and any number of children.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"abstract": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Prevents objects being created in this object type, leaving it as a holder of attributes for its children to inherit.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"inherited": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Passes this object type's attributes down to its child object types. Set it on the topmost object type and leave it unset below: the API turns it on for every descendant of an object type that has it. Forces replacement in either direction, since the API refuses to enable it on an object type that already has children and its payload cannot express disabling it.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"parent_object_type_inherited": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this object type was given a copy of its parent's attributes at creation, the UI's \"Add parent attributes\". Read-only: the object type payload cannot set it.",
			},
			"position": schema.Int64Attribute{
				Computed: true,
			},
			"created": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated": schema.StringAttribute{
				Computed: true,
			},
			"object_count": schema.Int64Attribute{
				Computed: true,
			},
			"attributes": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Plain attributes this object type owns, as attribute name to data type (" + strings.Join(sortedNames(dataTypes), ", ") +
					"). An attribute needing a label, a reference, uniqueness, cardinality or a description is a jiraassets_object_type_attribute resource instead; do not declare the same name both ways. Attributes shared by several object types belong on a parent with inherited = true rather than being repeated here.",
				Validators: []validator.Map{
					mapvalidator.ValueStringsAre(stringvalidator.OneOf(sortedNames(dataTypes)...)),
				},
			},
			"attribute_ids": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Attribute name to attribute id, for every entry in attributes. An attribute supplied by a parent reports the id it has on the object type that owns it, which is the id a value has to be written against.",
			},
		},
	}
}

// ModifyPlan resolves attribute_ids at plan time, so an attribute that already
// exists keeps its id in the plan instead of reading "known after apply".
func (r *objectTypeResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan objectTypeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired, unknown := attributeMapFromValue(plan.Attributes)
	if unknown {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, schemaPathAttributeIds, types.MapUnknown(types.StringType))...)
		return
	}

	// Terraform re-plans the create half of a replacement with a null prior state,
	// so this is not reached for one: a replacement discards every attribute with
	// the object type, and the ids in state would not survive it.
	currentIds := map[string]string{}
	if !req.State.Raw.IsNull() {
		var state objectTypeResourceModel
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}

		currentIds, _ = attributeMapFromValue(state.AttributeIds)
	}

	// A retype keeps the attribute, so only a new name gets an unknown id.
	ids := make(map[string]attr.Value, len(desired))
	for name := range desired {
		if id, ok := currentIds[name]; ok {
			ids[name] = types.StringValue(id)
			continue
		}

		ids[name] = types.StringUnknown()
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, schemaPathAttributeIds, types.MapValueMust(types.StringType, ids))...)
}

func (r *objectTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan objectTypeResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := &models.ObjectTypePayloadScheme{
		Name:               plan.Name.ValueString(),
		Description:        plan.Description.ValueString(),
		IconID:             plan.IconId.ValueString(),
		ObjectSchemaID:     plan.ObjectSchemaId.ValueString(),
		ParentObjectTypeID: plan.ParentObjectTypeId.ValueString(),
		AbstractObjectType: plan.AbstractObjectType.ValueBool(),
		Inherited:          plan.Inherited.ValueBool(),
	}

	objectType, response, err := r.client.ObjectType.Create(ctx, r.workspace_id, payload)
	if err != nil {
		logAPIError(ctx, "Error creating object type", response)

		resp.Diagnostics.AddError(
			"Error during object type creation",
			apiError(err, response),
		)
		return
	}

	// The object type exists from here on, so state is written even when the
	// attributes fail: dropping it would leak the object type.
	created := objectTypeModelFromScheme(objectType)
	created.Attributes = plan.Attributes

	// A descendant of an inherited object type is inherited whether it asked to
	// be or not. Say so, rather than letting the mismatch surface as a framework
	// "provider produced inconsistent result" bug report.
	if !plan.Inherited.IsUnknown() && plan.Inherited.ValueBool() != objectType.Inherited {
		resp.Diagnostics.AddAttributeError(
			schemaPathInherited,
			"Inheritance is decided by the parent object type",
			fmt.Sprintf("Object type %s was created with inherited = %t, but the API reports %t. Every descendant of an object type with inherited = true is inherited too. Leave inherited unset here and set it on the topmost object type.",
				objectType.ID, plan.Inherited.ValueBool(), objectType.Inherited),
		)
	}

	desired, _ := attributeMapFromValue(plan.Attributes)

	ids, diags := r.syncAttributes(ctx, created.Id.ValueString(), desired, nil)
	resp.Diagnostics.Append(diags...)

	// Reporting an error from a create that wrote state taints the resource, and a
	// replacement here would delete the object type along with every object in it.
	// The sync adopts whatever already exists, so a retry is safe.
	if diags.HasError() {
		resp.Diagnostics.AddWarning(
			"Object type created without all of its attributes",
			fmt.Sprintf("Object type %s exists and is recorded in state, but the error above taints it: the next apply would plan a replacement, deleting the object type and its objects. Untaint the resource and apply again instead — the attribute sync takes over what is already there.", created.Id.ValueString()),
		)
	}

	created.AttributeIds = attributeMapValue(ids)

	resp.Diagnostics.Append(resp.State.Set(ctx, created)...)
}

func (r *objectTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state objectTypeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	objectType, response, err := r.client.ObjectType.Get(ctx, r.workspace_id, state.Id.ValueString())
	if err != nil {
		logAPIError(ctx, "Error reading object type", response)

		resp.Diagnostics.AddError(
			"Error during object type reading",
			apiError(err, response),
		)
		return
	}

	currentIds, _ := attributeMapFromValue(state.AttributeIds)

	refreshed := objectTypeModelFromScheme(objectType)

	all, ids, diags := r.refreshAttributes(ctx, state.Id.ValueString(), currentIds)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// attributes is configuration-backed, so report what the API actually holds:
	// that is what makes a change made outside tofu show up as drift. A null
	// config stays null, since an empty map is a different value.
	refreshed.Attributes = attributeMapValue(all)
	if len(all) == 0 && state.Attributes.IsNull() {
		refreshed.Attributes = types.MapNull(types.StringType)
	}

	refreshed.AttributeIds = attributeMapValue(ids)

	resp.Diagnostics.Append(resp.State.Set(ctx, refreshed)...)
}

func (r *objectTypeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state objectTypeResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// PUT is a merge: only name and description are updatable in place, the
	// rest of the fields above require replacement.
	//
	// The name is sent only when it changes. The API's uniqueness check counts
	// the object type itself, so repeating its current name fails with "Name has
	// to be unique within the same object schema" — which would make every
	// attribute-only or description-only change unappliable.
	payload := &models.ObjectTypePayloadScheme{
		Description: plan.Description.ValueString(),
	}

	if plan.Name.ValueString() != state.Name.ValueString() {
		payload.Name = plan.Name.ValueString()
	}

	objectType, response, err := r.client.ObjectType.Update(ctx, r.workspace_id, plan.Id.ValueString(), payload)
	if err != nil {
		logAPIError(ctx, "Error updating object type", response)

		resp.Diagnostics.AddError(
			"Error during object type update",
			apiError(err, response),
		)
		return
	}

	updated := objectTypeModelFromScheme(objectType)
	updated.Attributes = plan.Attributes

	desired, _ := attributeMapFromValue(plan.Attributes)
	currentIds, _ := attributeMapFromValue(state.AttributeIds)

	ids, diags := r.syncAttributes(ctx, updated.Id.ValueString(), desired, currentIds)
	resp.Diagnostics.Append(diags...)

	updated.AttributeIds = attributeMapValue(ids)

	resp.Diagnostics.Append(resp.State.Set(ctx, updated)...)
}

func (r *objectTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state objectTypeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, response, err := r.client.ObjectType.Delete(ctx, r.workspace_id, state.Id.ValueString())
	if err != nil {
		logAPIError(ctx, "Error deleting object type", response)

		resp.Diagnostics.AddError(
			"Error during object type deletion",
			apiError(err, response),
		)
		return
	}
}

func (r *objectTypeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *objectTypeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req.ProviderData, &resp.Diagnostics)
}

func objectTypeModelFromScheme(o *models.ObjectTypeScheme) objectTypeResourceModel {
	iconId := ""
	if o.Icon != nil {
		iconId = o.Icon.ID
	}

	// parent_object_type_id is Optional (not Computed): map the API's
	// empty string back to null so an unset config does not drift.
	parentObjectTypeId := types.StringNull()
	if o.ParentObjectTypeID != "" {
		parentObjectTypeId = types.StringValue(o.ParentObjectTypeID)
	}

	return objectTypeResourceModel{
		WorkspaceId:        types.StringValue(o.WorkspaceID),
		GlobalId:           types.StringValue(o.GlobalID),
		Id:                 types.StringValue(o.ID),
		Name:               types.StringValue(o.Name),
		Description:        types.StringValue(o.Description),
		IconId:             types.StringValue(iconId),
		ObjectSchemaId:     types.StringValue(o.ObjectSchemaID),
		ParentObjectTypeId: parentObjectTypeId,
		AbstractObjectType: types.BoolValue(o.AbstractObjectType),
		Inherited:          types.BoolValue(o.Inherited),
		ParentInherited:    types.BoolValue(o.ParentObjectTypeInherited),
		Position:           types.Int64Value(int64(o.Position)),
		Created:            types.StringValue(o.Created),
		Updated:            types.StringValue(o.Updated),
		ObjectCount:        types.Int64Value(int64(o.ObjectCount)),
		Attributes:         types.MapNull(types.StringType),
		AttributeIds:       types.MapNull(types.StringType),
	}
}
