package handler

import (
	"encoding/json"
	"testing"
)

func TestValidSettingValue(t *testing.T) {
	tests := []struct {
		name        string
		settingType string
		value       string
		want        bool
	}{
		{name: "boolean", settingType: "boolean", value: `true`, want: true},
		{name: "boolean rejects string", settingType: "boolean", value: `"true"`},
		{name: "integer", settingType: "integer", value: `42`, want: true},
		{name: "integer rejects decimal", settingType: "integer", value: `4.2`},
		{name: "float", settingType: "float", value: `4.2`, want: true},
		{name: "string", settingType: "string", value: `"value"`, want: true},
		{name: "options array", settingType: "options", value: `["one","two"]`, want: true},
		{name: "options object", settingType: "options", value: `{"choices":["one"]}`, want: true},
		{name: "options rejects scalar", settingType: "options", value: `"one"`},
		{name: "unknown type", settingType: "unknown", value: `true`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validSettingValue(test.settingType, json.RawMessage(test.value)); got != test.want {
				t.Fatalf("validSettingValue(%q, %s) = %v, want %v", test.settingType, test.value, got, test.want)
			}
		})
	}
}
