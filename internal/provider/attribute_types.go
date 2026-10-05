package provider

import "sort"

// The Assets API encodes attribute types as bare integers. The config surface
// uses names instead, so a plan reads without the encoding table to hand and
// tofu rejects a bad value at plan time rather than the API rejecting it with a
// 400 at apply time.

// Attribute type names, as they appear in configuration.
const (
	attrTypeDefault         = "default"
	attrTypeObjectReference = "object_reference"
)

// attributeTypes maps a name to the wire value of the "type" field.
var attributeTypes = map[string]int64{
	attrTypeDefault:         0,
	attrTypeObjectReference: 1,
	"user":                  2,
	"group":                 4,
	"status":                7,
}

// dataTypes maps a name to the wire value of "defaultTypeId", which is only
// meaningful when type is default. Codes are from the Assets Cloud OpenAPI spec.
var dataTypes = map[string]int64{
	"text":     0,
	"integer":  1,
	"boolean":  2,
	"double":   3,
	"date":     4,
	"time":     5,
	"datetime": 6,
	"url":      7,
	"email":    8,
	"textarea": 9,
	"select":   10,
	"ip":       11,
}

// dataTypeSelect is the one data type that takes options.
const dataTypeSelect = "select"

// nameForCode reverses a wire value back to its configuration name, for Read.
func nameForCode(m map[string]int64, code int64) (string, bool) {
	for name, c := range m {
		if c == code {
			return name, true
		}
	}
	return "", false
}

// sortedNames keeps validator and documentation ordering stable across builds,
// which map iteration would not.
func sortedNames(m map[string]int64) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
