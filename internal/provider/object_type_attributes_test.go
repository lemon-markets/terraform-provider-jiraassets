package provider

import (
	"testing"

	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAttributeMapFromValue(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value       types.Map
		want        map[string]string
		wantUnknown bool
	}{
		"null": {
			value: types.MapNull(types.StringType),
			want:  map[string]string{},
		},
		"unknown map": {
			value:       types.MapUnknown(types.StringType),
			wantUnknown: true,
		},
		"unknown element": {
			value:       types.MapValueMust(types.StringType, map[string]attr.Value{"Region": types.StringUnknown()}),
			wantUnknown: true,
		},
		"known": {
			value: types.MapValueMust(types.StringType, map[string]attr.Value{"Region": types.StringValue("text")}),
			want:  map[string]string{"Region": "text"},
		},
	}

	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, unknown := attributeMapFromValue(test.value)
			if unknown != test.wantUnknown {
				t.Fatalf("unknown = %v, want %v", unknown, test.wantUnknown)
			}
			if unknown {
				return
			}
			if len(got) != len(test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
			for key, value := range test.want {
				if got[key] != value {
					t.Errorf("got[%q] = %q, want %q", key, got[key], value)
				}
			}
		})
	}
}

func TestInlineAttributeUpToDate(t *testing.T) {
	t.Parallel()

	text := &models.ObjectTypeAttributeScheme{
		Name:        "Region",
		Type:        int(attributeTypes[attrTypeDefault]),
		DefaultType: &models.ObjectTypeAssetAttributeDefaultTypeScheme{ID: int(dataTypes["text"])},
	}
	reference := &models.ObjectTypeAttributeScheme{
		Name: "Region",
		Type: int(attributeTypes[attrTypeObjectReference]),
	}

	tests := map[string]struct {
		existing *models.ObjectTypeAttributeScheme
		name     string
		dataType string
		want     bool
	}{
		"same name and data type":  {existing: text, name: "Region", dataType: "text", want: true},
		"different data type":      {existing: text, name: "Region", dataType: "integer"},
		"different name":           {existing: text, name: "AWS Region", dataType: "text"},
		"not a plain attribute":    {existing: reference, name: "Region", dataType: "text"},
		"case differs from wanted": {existing: text, name: "region", dataType: "text"},
	}

	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := inlineAttributeUpToDate(test.existing, test.name, test.dataType); got != test.want {
				t.Errorf("inlineAttributeUpToDate() = %v, want %v", got, test.want)
			}
		})
	}
}

// The built-in Name attribute holds the label, and the API rejects clearing the
// last label on an object type.
func TestInlineAttributePayloadKeepsLabel(t *testing.T) {
	t.Parallel()

	existing := &models.ObjectTypeAttributeScheme{Name: "Name", Label: true}

	payload, diags := inlineAttributePayload("Name", "text", existing)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !payload.Label {
		t.Error("payload.Label = false, want true when taking over a label attribute")
	}

	payload, diags = inlineAttributePayload("Region", "text", nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if payload.Label {
		t.Error("payload.Label = true, want false for a new attribute")
	}
	if payload.DefaultTypeID == nil || *payload.DefaultTypeID != int(dataTypes["text"]) {
		t.Errorf("payload.DefaultTypeID = %v, want %d", payload.DefaultTypeID, dataTypes["text"])
	}
}

func TestInlineAttributePayloadRejectsUnknownDataType(t *testing.T) {
	t.Parallel()

	if _, diags := inlineAttributePayload("Region", "geography", nil); !diags.HasError() {
		t.Error("expected an error for an unknown data type")
	}
}
