package metrics

import (
	"fmt"
	"reflect"
	"strings"
)

// LabelComposer lets you compose valid labels string
// Implement this interface if your want fast labels generation
// Otherwise it will fall back to a slow reflection-based implementation
type LabelComposer interface {
	ToLabelsString() string
}

// NameCompose returns a valid full metric name, composed of a metric name + stringified labels
// It accepts a valid LabelComposer interface, which is used to compose labels string.
//
// The NameCompose can be called for further GetOrCreateCounter/etc func:
//
//	// `my_counter{status="active",flag="false"}`
//	GetOrCreateCounter(NameCompose("my_counter", MyLabels{
//	  Status: "active",
//	  Flag:   false,
//	})).Inc()
func NameCompose(name string, lc LabelComposer) string {
	if lc == nil {
		return name
	}

	return name + lc.ToLabelsString()
}

//
// Auto composer
//

// labelComposerAutoMarker is just a marker interface.
// Interface is private so it's only be used via AutoLabelComposer implementation.
// This is made for safety reasons, so it's not allowed to pass a random struct to NameCompose() function.
type labelComposerAutoMarker interface {
	autoComposeMarker()
}

// AutoLabelComposer MUST be embedded in any struct that serves as a label composer.
// Embedding is required even if you provide custom implementation of LabelComposer (ToLabelsString() method)
type AutoLabelComposer struct{}

func (s AutoLabelComposer) autoComposeMarker() { panic("should never happen") }

// NameComposeAuto returns a valid full metric name, composed of a metric name + stringified labels
// It accepts a struct who embeds AutoLabelComposer so labels are generated from it.
//
// The NameComposeAuto can be called for further GetOrCreateCounter/etc func:
//
//	// `my_counter{status="active",flag="false"}`
//	GetOrCreateCounter(NameComposeAuto("my_counter", MyLabels{
//	  Status: "active",
//	  Flag:   false,
//	})).Inc()
func NameComposeAuto(name string, lc labelComposerAutoMarker) string {
	if lc == nil {
		return name
	}

	return name + reflectLabelCompose(lc)
}

// reflectLabelCompose composes labels string {field="value",...} from a struct
// It will use only exported scalar fields, and will skip fields with the `-` tag.
// By default, the snake_cased field name is used as the label name.
// Label's name can be overridden by using the `labels` tag
func reflectLabelCompose(lc labelComposerAutoMarker) string {
	labelsStr := "{"

	val := reflect.Indirect(reflect.ValueOf(lc))
	typ := val.Type()

	var n int
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous || !field.IsExported() {
			continue
		}
		ft := field.Type
		if field.Type.Kind() == reflect.Pointer {
			ft = field.Type.Elem()
		}
		fk := ft.Kind()

		// We only support basic scalar types: Strings, Numbers, Bool
		if fk != reflect.String && fk != reflect.Bool && (fk < reflect.Int || fk > reflect.Uint64) {
			continue
		}

		var labelName string
		if ourTag := field.Tag.Get(labelsTag); ourTag != "" {
			if ourTag == "-" { // tag="-" means "skip this field"
				continue
			}
			labelName = ourTag
		} else {
			labelName = toSnakeCase(field.Name)
		}

		if n > 0 {
			labelsStr += ","
		}
		labelsStr += labelName + `="` + stringifyLabelValue(val.Field(i)) + `"`
		n++
	}

	return labelsStr + "}"
}

// labelsTag is the tag name used for labels inside structs.
// The tag is optional, as if not present, field is used with snake_cased FieldName.
// It's useful to use a tag when you want to override the default naming or exclude a field from the metric.
var labelsTag = "labels"

// SetLabelsStructTag sets the tag name used for labels inside structs.
func SetLabelsStructTag(tag string) {
	labelsTag = tag
}

// stringifyLabelValue makes up a valid string value from a given field's value
// It's used ONLY in fallback reflect mode
// Field value might be a pointer, that's why we do reflect.Indirect()
// Note: in future we can handle default values here as well
func stringifyLabelValue(v reflect.Value) string {
	k := v.Kind()
	if k == reflect.Ptr {
		if v.IsNil() {
			return "nil"
		}
		v = v.Elem()
	}

	return fmt.Sprintf("%v", v.Interface())
}

// Convert struct field names to snake_case for Prometheus label compliance.
func toSnakeCase(s string) string {
	s = strings.TrimSpace(s)
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}
