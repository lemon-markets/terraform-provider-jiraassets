package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/ctreminiom/go-atlassian/v2/assets"
	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                   = &objectTypeAttributeResource{}
	_ resource.ResourceWithConfigure      = &objectTypeAttributeResource{}
	_ resource.ResourceWithImportState    = &objectTypeAttributeResource{}
	_ resource.ResourceWithValidateConfig = &objectTypeAttributeResource{}
)

// NewObjectTypeAttributeResource is a helper function to simplify the provider implementation.
func NewObjectTypeAttributeResource() resource.Resource {
	return &objectTypeAttributeResource{}
}

// objectTypeAttributeResource is the resource implementation.
type objectTypeAttributeResource struct {
	client       *assets.Client
	workspace_id string
}

func (r *objectTypeAttributeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_type_attribute"
}

type objectTypeAttributeResourceModel struct {
	WorkspaceId        types.String `tfsdk:"workspace_id"`
	GlobalId           types.String `tfsdk:"global_id"`
	Id                 types.String `tfsdk:"id"`
	ObjectTypeId       types.String `tfsdk:"object_type_id"`
	Name               types.String `tfsdk:"name"`
	Label              types.Bool   `tfsdk:"label"`
	Description        types.String `tfsdk:"description"`
	Type               types.String `tfsdk:"type"`
	DataType           types.String `tfsdk:"data_type"`
	TypeValue          types.String `tfsdk:"type_value"`
	ReferenceTypeId    types.String `tfsdk:"reference_type_id"`
	MinimumCardinality types.Int64  `tfsdk:"minimum_cardinality"`
	MaximumCardinality types.Int64  `tfsdk:"maximum_cardinality"`
	Suffix             types.String `tfsdk:"suffix"`
	UniqueAttribute    types.Bool   `tfsdk:"unique_attribute"`
	RegexValidation    types.String `tfsdk:"regex_validation"`
	IncludeChildren    types.Bool   `tfsdk:"include_child_object_types"`
	Removable          types.Bool   `tfsdk:"removable"`
}

var (
	schemaPathObjectTypeId = path.Root("object_type_id")
	schemaPathId           = path.Root("id")
	schemaPathType         = path.Root("type")
	schemaPathDataType     = path.Root("data_type")
	schemaPathTypeValue    = path.Root("type_value")

	schemaPathIncludeChildren = path.Root("include_child_object_types")
)

// Schema defines the schema for the resource.
func (r *objectTypeAttributeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Jira Assets object type attribute resource.",
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
				Description: "The ID of the attribute.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"object_type_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the object type this attribute belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the attribute.",
			},
			"label": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Marks this attribute as the object type's label. An object type has exactly one label, held at creation by the built-in Name attribute, so setting this adopts and renames that attribute instead of creating a second one: the API moves the label silently rather than erroring, which would otherwise leave Name behind as an unmanaged attribute.",
				PlanModifiers: []planmodifier.Bool{
					attributeLabelRequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(attrTypeDefault),
				Description: "The kind of attribute: " + strings.Join(sortedNames(attributeTypes), ", ") + ". Defaults to " + attrTypeDefault + ", which stores a value of data_type.",
				Validators: []validator.String{
					stringvalidator.OneOf(sortedNames(attributeTypes)...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"data_type": schema.StringAttribute{
				Optional:    true,
				Description: "The stored data type, required when type is " + attrTypeDefault + ": " + strings.Join(sortedNames(dataTypes), ", ") + ". The API accepts a change here even when existing attribute values are incompatible with the new type (e.g. text to integer on a non-numeric value) with no validation or migration, so a clean plan does not guarantee safe data.",
				Validators: []validator.String{
					stringvalidator.OneOf(sortedNames(dataTypes)...),
				},
			},
			"type_value": schema.StringAttribute{
				Optional:    true,
				Description: "For type = " + attrTypeObjectReference + ", the id of the target jiraassets_object_type. Required in that case.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"reference_type_id": schema.StringAttribute{
				Optional:    true,
				Description: "For type = " + attrTypeObjectReference + ", the id of the jiraassets_reference_type describing the relationship. Wire name is additionalValue.",
			},
			"minimum_cardinality": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "Minimum number of values for this attribute. Defaults to 0, matching the API.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"maximum_cardinality": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "Maximum number of values for this attribute, -1 for unbounded. Defaults to 1, matching the API.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"suffix": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"unique_attribute": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"regex_validation": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"include_child_object_types": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "For type = " + attrTypeObjectReference + ", accepts objects of the target object type's children as values too, not only the target itself. Wire name is includeChildObjectTypes.",
			},
			"removable": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the API allows deleting this attribute. False for the four attributes every object type is born with (Key, Created, Updated, Name); destroying such a resource drops it from state and leaves it in place.",
			},
		},
	}
}

// ValidateConfig rejects at plan time what the API would otherwise reject with a 400 at apply time.
func (r *objectTypeAttributeResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config objectTypeAttributeResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Type.IsUnknown() {
		return
	}

	// Null means the schema default applies.
	attrType := attrTypeDefault
	if !config.Type.IsNull() {
		attrType = config.Type.ValueString()
	}

	switch attrType {
	case attrTypeDefault:
		if config.DataType.IsNull() && !config.DataType.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				schemaPathDataType,
				"Missing data_type",
				fmt.Sprintf("data_type is required when type is %s.", attrTypeDefault),
			)
		}
	case attrTypeObjectReference:
		if config.TypeValue.IsNull() && !config.TypeValue.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				schemaPathTypeValue,
				"Missing type_value",
				fmt.Sprintf("type_value is required when type is %s: it must hold the target object type's id.", attrTypeObjectReference),
			)
		}
	}

	// data_type is silently ignored by the API for every other type, which would
	// leave a value in the config that does nothing.
	if attrType != attrTypeDefault && !config.DataType.IsNull() && !config.DataType.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			schemaPathDataType,
			"Unexpected data_type",
			fmt.Sprintf("data_type only applies when type is %s, but type is %s.", attrTypeDefault, attrType),
		)
	}

	// Nothing to descend into when the attribute does not point at an object type.
	if attrType != attrTypeObjectReference && config.IncludeChildren.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			schemaPathIncludeChildren,
			"Unexpected include_child_object_types",
			fmt.Sprintf("include_child_object_types only applies when type is %s, but type is %s.", attrTypeObjectReference, attrType),
		)
	}
}

// Create adopts an existing non-removable attribute where one collides, and
// creates a new attribute otherwise.
//
// Every object type is born with Key, Created, Updated and Name. None of the four
// can be deleted, and Name holds the label. Creating a second label attribute
// makes the API move the label silently rather than erroring, leaving Name behind
// as an unmanaged attribute the config never asked for. Renaming Name in place
// instead keeps one attribute, managed, with the intended name.
func (r *objectTypeAttributeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan objectTypeAttributeResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := objectTypeAttributePayloadFromModel(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	adoptee, diags := r.findAdoptee(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var (
		attribute *models.ObjectTypeAttributeScheme
		response  *models.ResponseScheme
		err       error
	)

	if adoptee != nil {
		tflog.Info(ctx, "Adopting built-in object type attribute", map[string]interface{}{
			"object_type_id": plan.ObjectTypeId.ValueString(),
			"attribute_id":   adoptee.ID,
			"existing_name":  adoptee.Name,
			"name":           plan.Name.ValueString(),
		})
		attribute, response, err = r.client.ObjectTypeAttribute.Update(ctx, r.workspace_id, plan.ObjectTypeId.ValueString(), adoptee.ID, payload)
	} else {
		attribute, response, err = r.client.ObjectTypeAttribute.Create(ctx, r.workspace_id, plan.ObjectTypeId.ValueString(), payload)
	}

	if err != nil {
		logAPIError(ctx, "Error creating object type attribute", response)

		resp.Diagnostics.AddError(
			"Error during object type attribute creation",
			apiError(err, response),
		)
		return
	}

	plan, diags = objectTypeAttributeModelFromScheme(attribute, plan.ObjectTypeId)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// findAdoptee returns the built-in attribute this resource should take over
// instead of creating a new one, or nil to create.
//
// Only non-removable attributes are candidates: those are exactly the four the
// API creates on its own, so adoption can never quietly capture an attribute
// another resource manages.
func (r *objectTypeAttributeResource) findAdoptee(ctx context.Context, plan objectTypeAttributeResourceModel) (*models.ObjectTypeAttributeScheme, diag.Diagnostics) {
	objectTypeId := plan.ObjectTypeId.ValueString()

	inventory, diags := attributeInventoryFor(ctx, r.client, r.workspace_id, objectTypeId)
	if diags.HasError() {
		return nil, diags
	}

	// An inherited name is already taken. The API rejects creating it, and it can
	// only be changed on the object type that owns it.
	if inherited := inventory.findInherited(plan.Name.ValueString()); inherited != nil {
		diags.AddError(
			"Attribute name is inherited from a parent object type",
			fmt.Sprintf("Object type %s inherits attribute %q (id %s) from object type %s, so it cannot hold one of its own by that name. Manage it on the object type that owns it, or rename this one.", objectTypeId, inherited.Name, inherited.ID, inherited.ObjectType.ID),
		)
		return nil, diags
	}

	// "Impossible to set child's attribute as label if inheritance": the label
	// belongs to whichever object type owns the attributes.
	if plan.Label.ValueBool() && inventory.inheritedLabel != nil {
		diags.AddError(
			"Cannot label an attribute on an inherited child object type",
			fmt.Sprintf("Object type %s inherits its attributes from object type %s, which holds the label as %q (id %s). The API refuses a label on the child. Declare the label on the object type that owns the attributes; every child shows it.",
				objectTypeId, inventory.inheritedLabel.ObjectType.ID, inventory.inheritedLabel.Name, inventory.inheritedLabel.ID),
		)
		return nil, diags
	}

	var sameName, labelHolder *models.ObjectTypeAttributeScheme
	for _, attribute := range inventory.ownById {
		if attribute.Removable {
			continue
		}
		if strings.EqualFold(attribute.Name, plan.Name.ValueString()) {
			sameName = attribute
		}
		if attribute.Label {
			labelHolder = attribute
		}
	}

	// A name match wins: the config is asking to manage that attribute by name.
	candidate := sameName
	if candidate == nil && plan.Label.ValueBool() && labelHolder != nil {
		candidate = labelHolder
	}
	if candidate == nil {
		return nil, diags
	}

	// Key, Created and Updated are not editable, so a PUT would 400. Say why here
	// rather than surfacing the API's bare error.
	if !candidate.Editable {
		diags.AddError(
			"Cannot manage a read-only built-in attribute",
			fmt.Sprintf("Attribute %q (id %s) on object type %s is created by the API and is not editable, so tofu cannot manage it. Remove this resource from the configuration.", candidate.Name, candidate.ID, plan.ObjectTypeId.ValueString()),
		)
		return nil, diags
	}

	return candidate, diags
}

// Read has no single-object GET on this API; list the parent object type's
// attributes and match by id. If absent, the attribute was deleted out of band.
func (r *objectTypeAttributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state objectTypeAttributeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	inventory, diags := attributeInventoryFor(ctx, r.client, r.workspace_id, state.ObjectTypeId.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if attribute, ok := inventory.ownById[state.Id.ValueString()]; ok {
		state, diags = objectTypeAttributeModelFromScheme(attribute, state.ObjectTypeId)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		return
	}

	// Attribute no longer exists on the object type; drop it from state.
	resp.State.RemoveResource(ctx)
}

func (r *objectTypeAttributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan objectTypeAttributeResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := objectTypeAttributePayloadFromModel(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	attribute, response, err := r.client.ObjectTypeAttribute.Update(ctx, r.workspace_id, plan.ObjectTypeId.ValueString(), plan.Id.ValueString(), payload)
	if err != nil {
		logAPIError(ctx, "Error updating object type attribute", response)

		resp.Diagnostics.AddError(
			"Error during object type attribute update",
			apiError(err, response),
		)
		return
	}

	plan, diags = objectTypeAttributeModelFromScheme(attribute, plan.ObjectTypeId)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete skips the API call for adopted built-ins: the API returns 400 for those,
// and they stop being managed rather than stopping existing.
func (r *objectTypeAttributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state objectTypeAttributeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !state.Removable.IsNull() && !state.Removable.ValueBool() {
		tflog.Warn(ctx, "Dropping non-removable object type attribute from state without deleting it", map[string]interface{}{
			"object_type_id": state.ObjectTypeId.ValueString(),
			"attribute_id":   state.Id.ValueString(),
			"name":           state.Name.ValueString(),
		})
		return
	}

	response, err := r.client.ObjectTypeAttribute.Delete(ctx, r.workspace_id, state.Id.ValueString())
	if err != nil {
		logAPIError(ctx, "Error deleting object type attribute", response)

		resp.Diagnostics.AddError(
			"Error during object type attribute deletion",
			apiError(err, response),
		)
		return
	}
}

// ImportState takes a composite id "objectTypeId/attributeId" since Read needs
// the parent object type id and there is no single-attribute GET to recover it from.
func (r *objectTypeAttributeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier in the form \"objectTypeId/attributeId\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, schemaPathObjectTypeId, parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, schemaPathId, parts[1])...)
}

func (r *objectTypeAttributeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func objectTypeAttributePayloadFromModel(m objectTypeAttributeResourceModel) (*models.ObjectTypeAttributePayloadScheme, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrType, ok := attributeTypes[m.Type.ValueString()]
	if !ok {
		diags.AddAttributeError(
			schemaPathType,
			"Unknown attribute type",
			fmt.Sprintf("%q is not a known attribute type. Valid values: %s.", m.Type.ValueString(), strings.Join(sortedNames(attributeTypes), ", ")),
		)
		return nil, diags
	}

	wireType := int(attrType)
	minimum := int(m.MinimumCardinality.ValueInt64())
	maximum := int(m.MaximumCardinality.ValueInt64())

	payload := &models.ObjectTypeAttributePayloadScheme{
		Name:               m.Name.ValueString(),
		Label:              m.Label.ValueBool(),
		Description:        m.Description.ValueString(),
		Type:               &wireType,
		TypeValue:          m.TypeValue.ValueString(),
		AdditionalValue:    m.ReferenceTypeId.ValueString(),
		MinimumCardinality: &minimum,
		MaximumCardinality: &maximum,
		Suffix:             m.Suffix.ValueString(),
		UniqueAttribute:    m.UniqueAttribute.ValueBool(),
		RegexValidation:    m.RegexValidation.ValueString(),

		IncludeChildObjectTypes: m.IncludeChildren.ValueBool(),
	}

	// Only meaningful for the default type, and the API treats 0 (text) as a
	// significant value, so omit the pointer entirely rather than sending zero.
	if !m.DataType.IsNull() {
		dataType, ok := dataTypes[m.DataType.ValueString()]
		if !ok {
			diags.AddAttributeError(
				schemaPathDataType,
				"Unknown data type",
				fmt.Sprintf("%q is not a known data type. Valid values: %s.", m.DataType.ValueString(), strings.Join(sortedNames(dataTypes), ", ")),
			)
			return nil, diags
		}
		wireDataType := int(dataType)
		payload.DefaultTypeID = &wireDataType
	}

	return payload, diags
}

// objectTypeAttributeModelFromScheme maps a Read/Create/Update response back onto
// the config surface. Create and Read use different wire fields for reference
// attributes (typeValue/additionalValue on write, referenceObjectTypeId/referenceType on
// read), so this is the one place that has to reconcile them.
func objectTypeAttributeModelFromScheme(a *models.ObjectTypeAttributeScheme, objectTypeId types.String) (objectTypeAttributeResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	typeValue := a.TypeValue
	if a.ReferenceObjectTypeID != "" {
		typeValue = a.ReferenceObjectTypeID
	}

	referenceTypeId := ""
	if a.ReferenceType != nil {
		// ObjectTypeAssetAttributeReferenceTypeScheme has no ID field; recover
		// it from globalId, which is "<workspaceId>:<referenceTypeId>".
		if idx := strings.LastIndex(a.ReferenceType.GlobalID, ":"); idx != -1 {
			referenceTypeId = a.ReferenceType.GlobalID[idx+1:]
		}
	}

	attrType, ok := nameForCode(attributeTypes, int64(a.Type))
	if !ok {
		diags.AddError(
			"Unsupported attribute type",
			fmt.Sprintf("Attribute %q (id %s) has type %d, which this provider does not model. Please report this issue to the provider developers.", a.Name, a.ID, a.Type),
		)
		return objectTypeAttributeResourceModel{}, diags
	}

	dataType := types.StringNull()
	if a.DefaultType != nil {
		name, ok := nameForCode(dataTypes, int64(a.DefaultType.ID))
		if !ok {
			diags.AddError(
				"Unsupported data type",
				fmt.Sprintf("Attribute %q (id %s) has data type %d (%s), which this provider does not model. Please report this issue to the provider developers.", a.Name, a.ID, a.DefaultType.ID, a.DefaultType.Name),
			)
			return objectTypeAttributeResourceModel{}, diags
		}
		dataType = types.StringValue(name)
	}

	return objectTypeAttributeResourceModel{
		WorkspaceId:        types.StringValue(a.WorkspaceID),
		GlobalId:           types.StringValue(a.GlobalID),
		Id:                 types.StringValue(a.ID),
		ObjectTypeId:       objectTypeId,
		Name:               types.StringValue(a.Name),
		Label:              types.BoolValue(a.Label),
		Description:        types.StringValue(a.Description),
		Type:               types.StringValue(attrType),
		DataType:           dataType,
		TypeValue:          optionalStringValue(typeValue),
		ReferenceTypeId:    optionalStringValue(referenceTypeId),
		MinimumCardinality: types.Int64Value(int64(a.MinimumCardinality)),
		MaximumCardinality: types.Int64Value(int64(a.MaximumCardinality)),
		Suffix:             types.StringValue(a.Suffix),
		UniqueAttribute:    types.BoolValue(a.UniqueAttribute),
		RegexValidation:    types.StringValue(a.RegexValidation),
		IncludeChildren:    types.BoolValue(a.IncludeChildObjectTypes),
		Removable:          types.BoolValue(a.Removable),
	}, diags
}

func optionalStringValue(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// attributeLabelRequiresReplace requires replacement only when label flips
// true -> false: the API returns 400 in that direction ("at least one label
// must be set on the object type") but allows false -> true in place.
func attributeLabelRequiresReplace() planmodifier.Bool {
	return labelRequiresReplaceModifier{}
}

type labelRequiresReplaceModifier struct{}

func (m labelRequiresReplaceModifier) Description(_ context.Context) string {
	return "Requires replacement if label changes from true to false."
}

func (m labelRequiresReplaceModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m labelRequiresReplaceModifier) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	if req.StateValue.ValueBool() && !req.PlanValue.ValueBool() {
		resp.RequiresReplace = true
	}
}
