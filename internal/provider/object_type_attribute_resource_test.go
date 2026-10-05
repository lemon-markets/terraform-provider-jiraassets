package provider

import (
	"context"
	"testing"

	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// attributeConfig builds a config from the resource schema, leaving every
// attribute not given null.
func attributeConfig(t *testing.T, values map[string]tftypes.Value) tfsdk.Config {
	t.Helper()

	r := &objectTypeAttributeResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)

	objectType := schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	all := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, typ := range objectType.AttributeTypes {
		if v, ok := values[name]; ok {
			all[name] = v
			continue
		}
		all[name] = tftypes.NewValue(typ, nil)
	}

	return tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, all)}
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func strList(values ...string) tftypes.Value {
	elements := make([]tftypes.Value, len(values))
	for i, v := range values {
		elements[i] = str(v)
	}
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, elements)
}

func TestObjectTypeAttributeValidateConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		values  map[string]tftypes.Value
		wantErr bool
	}{
		"plain text": {
			values: map[string]tftypes.Value{"data_type": str("text")},
		},
		"select with options": {
			values: map[string]tftypes.Value{"data_type": str("select"), "options": strList("Low", "High")},
		},
		"select without options": {
			values:  map[string]tftypes.Value{"data_type": str("select")},
			wantErr: true,
		},
		"options on text": {
			values:  map[string]tftypes.Value{"data_type": str("text"), "options": strList("Low")},
			wantErr: true,
		},
		"type_value on default": {
			values:  map[string]tftypes.Value{"data_type": str("text"), "type_value": str("12")},
			wantErr: true,
		},
		"reference_type_id on user": {
			values:  map[string]tftypes.Value{"type": str("user"), "reference_type_id": str("3")},
			wantErr: true,
		},
		"reference with type_value": {
			values: map[string]tftypes.Value{"type": str(attrTypeObjectReference), "type_value": str("12"), "reference_type_id": str("3")},
		},
		"unique with default cardinality": {
			values: map[string]tftypes.Value{"data_type": str("text"), "unique_attribute": tftypes.NewValue(tftypes.Bool, true)},
		},
		"unique multi-valued": {
			values: map[string]tftypes.Value{
				"data_type":           str("text"),
				"unique_attribute":    tftypes.NewValue(tftypes.Bool, true),
				"maximum_cardinality": tftypes.NewValue(tftypes.Number, -1),
			},
			wantErr: true,
		},
	}

	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := resource.ValidateConfigRequest{Config: attributeConfig(t, test.values)}
			var resp resource.ValidateConfigResponse
			(&objectTypeAttributeResource{}).ValidateConfig(context.Background(), req, &resp)

			if got := resp.Diagnostics.HasError(); got != test.wantErr {
				t.Errorf("HasError() = %v, want %v: %v", got, test.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestSelectOptionPattern(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]bool{
		"High":       true,
		"eu-west-1":  true,
		"Very high":  true,
		"a":          true,
		"High,Low":   false,
		" High":      false,
		"High ":      false,
		"":           false,
		"line\nfeed": true,
	} {
		if got := selectOptionPattern.MatchString(value); got != want {
			t.Errorf("selectOptionPattern.MatchString(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestObjectTypeAttributeOptionsRoundTrip(t *testing.T) {
	t.Parallel()

	model := objectTypeAttributeResourceModel{
		Type:               types.StringValue(attrTypeDefault),
		DataType:           types.StringValue(dataTypeSelect),
		Options:            types.ListValueMust(types.StringType, []attr.Value{types.StringValue("Low"), types.StringValue("High")}),
		MinimumCardinality: types.Int64Value(0),
		MaximumCardinality: types.Int64Value(1),
	}

	payload, diags := objectTypeAttributePayloadFromModel(model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if payload.Options != "Low,High" {
		t.Errorf("payload.Options = %q, want %q", payload.Options, "Low,High")
	}

	read, diags := objectTypeAttributeModelFromScheme(&models.ObjectTypeAttributeScheme{
		Type:        int(attributeTypes[attrTypeDefault]),
		DefaultType: &models.ObjectTypeAssetAttributeDefaultTypeScheme{ID: int(dataTypes[dataTypeSelect])},
		Options:     "Low, High",
	}, types.StringValue("1"))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !read.Options.Equal(model.Options) {
		t.Errorf("read.Options = %v, want %v", read.Options, model.Options)
	}

	// Options left behind by a retype away from select are not reported.
	read, _ = objectTypeAttributeModelFromScheme(&models.ObjectTypeAttributeScheme{
		Type:        int(attributeTypes[attrTypeDefault]),
		DefaultType: &models.ObjectTypeAssetAttributeDefaultTypeScheme{ID: int(dataTypes["text"])},
		Options:     "Low,High",
	}, types.StringValue("1"))
	if !read.Options.IsNull() {
		t.Errorf("read.Options = %v, want null for a text attribute", read.Options)
	}
}

func TestAttributeInventoryIdsByName(t *testing.T) {
	t.Parallel()

	inventory := &attributeInventory{visibleById: map[string]*models.ObjectTypeAttributeScheme{
		"10": {ID: "10", Name: "Name"},
		"11": {ID: "11", Name: "Region", ObjectType: &models.ObjectTypeScheme{ID: "parent"}},
	}}

	got := inventory.idsByName()
	if len(got) != 2 || got["Name"] != "10" || got["Region"] != "11" {
		t.Errorf("idsByName() = %v", got)
	}
}

func TestAttributeInventoryOtherUniqueAttributes(t *testing.T) {
	t.Parallel()

	inventory := &attributeInventory{visibleById: map[string]*models.ObjectTypeAttributeScheme{
		"10": {ID: "10", Name: "Name"},
		"11": {ID: "11", Name: "Serial", UniqueAttribute: true},
		"12": {ID: "12", Name: "Asset Tag", UniqueAttribute: true, ObjectType: &models.ObjectTypeScheme{ID: "parent"}},
	}}

	if got := inventory.otherUniqueAttributes(""); len(got) != 2 {
		t.Errorf("otherUniqueAttributes(\"\") = %v, want the own and the inherited one", got)
	}
	if got := inventory.otherUniqueAttributes("11"); len(got) != 1 || got[0] != `"Asset Tag" (id 12)` {
		t.Errorf("otherUniqueAttributes(\"11\") = %v", got)
	}
}
