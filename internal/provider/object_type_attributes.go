package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/ctreminiom/go-atlassian/v2/assets"
	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// attributeMapFromValue reads a map(string) out of config, plan or state. The
// second return reports that some part of it is unknown, so callers can leave the
// merge to apply rather than planning a half-known value.
func attributeMapFromValue(m types.Map) (map[string]string, bool) {
	if m.IsUnknown() {
		return nil, true
	}

	out := make(map[string]string, len(m.Elements()))
	for name, value := range m.Elements() {
		dataType, ok := value.(types.String)
		if !ok || dataType.IsUnknown() {
			return nil, true
		}
		out[name] = dataType.ValueString()
	}

	return out, false
}

func attributeMapValue(m map[string]string) types.Map {
	elements := make(map[string]attr.Value, len(m))
	for name, value := range m {
		elements[name] = types.StringValue(value)
	}

	return types.MapValueMust(types.StringType, elements)
}

// attributeInventory is everything one object type can see, split by who owns it.
//
// A child of an object type with inherited = true owns nothing at all — not even
// the built-in Key, Created, Updated and Name, which stay with the parent. Both
// halves matter: only an owned attribute may be written, and an inherited name
// still occupies the namespace, so creating it fails on "Name has to be unique!".
type attributeInventory struct {
	visibleById   map[string]*models.ObjectTypeAttributeScheme
	ownById       map[string]*models.ObjectTypeAttributeScheme
	ownByName     map[string]*models.ObjectTypeAttributeScheme
	inheritedName map[string]*models.ObjectTypeAttributeScheme

	// inheritedLabel is the parent's label attribute, when the label is not this
	// object type's to move.
	inheritedLabel *models.ObjectTypeAttributeScheme
}

// attributeInventoryFor lists an object type's attributes without
// excludeParentAttributes, so inherited names stay visible, and splits them on
// the object type each one reports as its owner.
func attributeInventoryFor(ctx context.Context, client *assets.Client, workspaceId, objectTypeId string) (*attributeInventory, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes, response, err := client.ObjectType.Attributes(ctx, workspaceId, objectTypeId, nil)
	if err != nil {
		logAPIError(ctx, "Error listing object type attributes", response)
		diags.AddError(
			"Error listing object type attributes",
			fmt.Sprintf("Could not list the attributes of object type %s: %s", objectTypeId, apiError(err, response)),
		)
		return nil, diags
	}

	inventory := &attributeInventory{
		visibleById:   make(map[string]*models.ObjectTypeAttributeScheme, len(attributes)),
		ownById:       make(map[string]*models.ObjectTypeAttributeScheme, len(attributes)),
		ownByName:     make(map[string]*models.ObjectTypeAttributeScheme, len(attributes)),
		inheritedName: make(map[string]*models.ObjectTypeAttributeScheme, len(attributes)),
	}

	for _, attribute := range attributes {
		inventory.visibleById[attribute.ID] = attribute

		// An attribute that names no owner counts as owned: guessing "inherited"
		// would stop the provider managing one it is responsible for.
		if attribute.ObjectType != nil && attribute.ObjectType.ID != "" && attribute.ObjectType.ID != objectTypeId {
			inventory.inheritedName[strings.ToLower(attribute.Name)] = attribute
			if attribute.Label {
				inventory.inheritedLabel = attribute
			}
			continue
		}

		inventory.ownById[attribute.ID] = attribute
		inventory.ownByName[strings.ToLower(attribute.Name)] = attribute
	}

	return inventory, diags
}

// findOwn returns the owned attribute a name refers to, preferring the id already
// in state so a rename follows the attribute instead of replacing it.
func (i *attributeInventory) findOwn(id, name string) *models.ObjectTypeAttributeScheme {
	if attribute, ok := i.ownById[id]; ok {
		return attribute
	}

	return i.ownByName[strings.ToLower(name)]
}

// idsByName maps every visible attribute's name, as the API holds it, to its id.
func (i *attributeInventory) idsByName() map[string]string {
	ids := make(map[string]string, len(i.visibleById))
	for id, attribute := range i.visibleById {
		ids[attribute.Name] = id
	}
	return ids
}

func (i *attributeInventory) findInherited(name string) *models.ObjectTypeAttributeScheme {
	return i.inheritedName[strings.ToLower(name)]
}

// syncAttributes reconciles the attributes an object type owns inline with the
// merged configuration and returns attribute name to id.
//
// Attributes are separate API objects, so unlike an AWS tag these have to be
// created, retyped and deleted one call at a time. An attribute whose name
// already exists on the object type is taken over rather than duplicated: the
// API accepts two attributes with the same name, and the second one would then
// shadow the first everywhere a name is used.
//
// The ids collected so far come back even on error, so a partly applied create
// can be written to state instead of leaking.
func (r *objectTypeResource) syncAttributes(ctx context.Context, objectTypeId string, desired, currentIds map[string]string) (map[string]string, diag.Diagnostics) {
	ids := make(map[string]string, len(desired))

	inventory, diags := attributeInventoryFor(ctx, r.client, r.workspace_id, objectTypeId)
	if diags.HasError() {
		return ids, diags
	}

	for name, dataType := range desired {
		existing := inventory.findOwn(currentIds[name], name)

		// The parent already supplies this name. Creating it would fail on
		// uniqueness and it cannot be edited from here, so record the parent's
		// id: that is the id anything writing a value has to use.
		if existing == nil {
			if inherited := inventory.findInherited(name); inherited != nil {
				inheritedType, ok := inlineAttributeDataType(inherited)
				if !ok || inheritedType != dataType {
					diags.AddError(
						"Attribute is inherited with a different definition",
						fmt.Sprintf("Object type %s inherits attribute %q (id %s) from object type %s, where it is %s rather than %s. Change it on the object type that owns it or drop it from this one: retyping it here would change every object type that inherits it.",
							objectTypeId, inherited.Name, inherited.ID, inherited.ObjectType.ID, describeAttributeType(inherited), dataType),
					)
					return ids, diags
				}

				tflog.Debug(ctx, "Reusing an inherited object type attribute", map[string]interface{}{
					"object_type_id": objectTypeId,
					"attribute_id":   inherited.ID,
					"name":           inherited.Name,
					"owned_by":       inherited.ObjectType.ID,
				})

				ids[name] = inherited.ID
				continue
			}
		}

		payload, payloadDiags := inlineAttributePayload(name, dataType, existing)
		diags.Append(payloadDiags...)
		if diags.HasError() {
			return ids, diags
		}

		if existing == nil {
			attribute, response, err := r.client.ObjectTypeAttribute.Create(ctx, r.workspace_id, objectTypeId, payload)
			if err != nil {
				logAPIError(ctx, "Error creating object type attribute", response)
				diags.AddError(
					"Error creating object type attribute",
					fmt.Sprintf("Could not create attribute %q on object type %s: %s", name, objectTypeId, apiError(err, response)),
				)
				return ids, diags
			}

			ids[name] = attribute.ID
			continue
		}

		ids[name] = existing.ID

		if inlineAttributeUpToDate(existing, name, dataType) {
			continue
		}

		if !existing.Editable {
			diags.AddError(
				"Cannot manage a read-only built-in attribute",
				fmt.Sprintf("Attribute %q (id %s) on object type %s is created by the API and is not editable, so tofu cannot manage it. Remove it from attributes.", existing.Name, existing.ID, objectTypeId),
			)
			return ids, diags
		}

		if _, response, err := r.client.ObjectTypeAttribute.Update(ctx, r.workspace_id, objectTypeId, existing.ID, payload); err != nil {
			logAPIError(ctx, "Error updating object type attribute", response)
			diags.AddError(
				"Error updating object type attribute",
				fmt.Sprintf("Could not update attribute %q (id %s) on object type %s: %s", name, existing.ID, objectTypeId, apiError(err, response)),
			)
			return ids, diags
		}
	}

	kept := make(map[string]bool, len(ids))
	for _, id := range ids {
		kept[id] = true
	}

	for name, id := range currentIds {
		if _, wanted := desired[name]; wanted {
			continue
		}

		// An attribute matched by name under a different key is still managed.
		if kept[id] {
			continue
		}

		// Never delete a parent's attribute. Dropping an inherited name from the
		// configuration stops tracking it; the parent still supplies it.
		existing, ok := inventory.ownById[id]
		if !ok {
			continue
		}

		if !existing.Removable {
			tflog.Warn(ctx, "Leaving non-removable object type attribute in place", map[string]interface{}{
				"object_type_id": objectTypeId,
				"attribute_id":   id,
				"name":           name,
			})
			continue
		}

		if response, err := r.client.ObjectTypeAttribute.Delete(ctx, r.workspace_id, id); err != nil {
			logAPIError(ctx, "Error deleting object type attribute", response)
			diags.AddError(
				"Error deleting object type attribute",
				fmt.Sprintf("Could not delete attribute %q (id %s) on object type %s: %s", name, id, objectTypeId, apiError(err, response)),
			)
			return ids, diags
		}
	}

	return ids, diags
}

// refreshAttributes re-reads the attributes an object type has, keyed by the name
// the API currently holds. Inherited ones count: they are visible from here, and
// their ids are what a value has to be written against. One deleted or retyped
// out of band drops out, which is what puts it back in the plan.
//
// The third map is every visible attribute, for all_attribute_ids.
func (r *objectTypeResource) refreshAttributes(ctx context.Context, objectTypeId string, currentIds map[string]string) (map[string]string, map[string]string, map[string]string, diag.Diagnostics) {
	all := map[string]string{}
	ids := map[string]string{}

	inventory, diags := attributeInventoryFor(ctx, r.client, r.workspace_id, objectTypeId)
	if diags.HasError() {
		return all, ids, map[string]string{}, diags
	}

	for _, id := range currentIds {
		attribute, ok := inventory.visibleById[id]
		if !ok {
			continue
		}

		dataType, ok := inlineAttributeDataType(attribute)
		if !ok {
			continue
		}

		all[attribute.Name] = dataType
		ids[attribute.Name] = id
	}

	return all, ids, inventory.idsByName(), diags
}

// inlineAttributePayload keeps the existing label flag: the API returns 400 when
// the last label on an object type is cleared, and taking over the built-in Name
// attribute would otherwise do exactly that.
func inlineAttributePayload(name, dataType string, existing *models.ObjectTypeAttributeScheme) (*models.ObjectTypeAttributePayloadScheme, diag.Diagnostics) {
	var diags diag.Diagnostics

	wireDataType, ok := dataTypes[dataType]
	if !ok {
		diags.AddError(
			"Unknown data type",
			fmt.Sprintf("%q is not a known data type for attribute %q. Valid values: %s.", dataType, name, strings.Join(sortedNames(dataTypes), ", ")),
		)
		return nil, diags
	}

	wireType := int(attributeTypes[attrTypeDefault])
	wireDefaultType := int(wireDataType)

	payload := &models.ObjectTypeAttributePayloadScheme{
		Name:          name,
		Type:          &wireType,
		DefaultTypeID: &wireDefaultType,
	}

	if existing != nil {
		payload.Label = existing.Label
	}

	return payload, diags
}

// inlineAttributeUpToDate reports whether an attribute already matches what the
// inline configuration asks for. Cardinality, uniqueness and description are not
// compared: an inline attribute does not set them, so an attribute that once had
// them set by hand is left alone rather than reset on every apply.
func inlineAttributeUpToDate(existing *models.ObjectTypeAttributeScheme, name, dataType string) bool {
	if existing.Name != name {
		return false
	}

	current, ok := inlineAttributeDataType(existing)

	return ok && current == dataType
}

// inlineAttributeDataType returns the data type name of a plain attribute. It
// reports false for references, users, groups and statuses, none of which an
// inline attribute can describe.
func inlineAttributeDataType(attribute *models.ObjectTypeAttributeScheme) (string, bool) {
	if int64(attribute.Type) != attributeTypes[attrTypeDefault] || attribute.DefaultType == nil {
		return "", false
	}

	return nameForCode(dataTypes, int64(attribute.DefaultType.ID))
}

// describeAttributeType names an attribute's type for a diagnostic, falling back
// to the wire value for the kinds an inline attribute cannot express.
func describeAttributeType(attribute *models.ObjectTypeAttributeScheme) string {
	if dataType, ok := inlineAttributeDataType(attribute); ok {
		return dataType
	}

	if name, ok := nameForCode(attributeTypes, int64(attribute.Type)); ok {
		return name
	}

	return fmt.Sprintf("type %d", attribute.Type)
}
