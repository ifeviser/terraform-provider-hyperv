package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestHyperVVhdDiff_SourceChangeReplacement(t *testing.T) {
	t.Parallel()

	const vhdPath = `C:\VMs\test\test.vhdx`
	srcA := PathStateFunc(`C:\golden\a.vhdx`)
	srcB := PathStateFunc(`C:\golden\b.vhdx`)

	testCases := []struct {
		name            string
		key             string
		stateValue      string
		configValue     interface{}
		wantChange      bool
		wantRequiresNew bool
	}{
		{name: "source changed", key: "source", stateValue: srcA, configValue: `C:\golden\b.vhdx`, wantChange: true, wantRequiresNew: true},
		{name: "source adopted after import", key: "source", stateValue: "", configValue: `C:\golden\a.vhdx`, wantChange: true, wantRequiresNew: false},
		{name: "source unchanged", key: "source", stateValue: srcB, configValue: `C:\golden\b.vhdx`, wantChange: false},
		{name: "source_vm changed", key: "source_vm", stateValue: "vm-a", configValue: "vm-b", wantChange: true, wantRequiresNew: true},
		{name: "source_vm adopted after import", key: "source_vm", stateValue: "", configValue: "vm-a", wantChange: true, wantRequiresNew: false},
		{name: "source_disk changed", key: "source_disk", stateValue: "1", configValue: 2, wantChange: true, wantRequiresNew: true},
		{name: "source_disk adopted after import", key: "source_disk", stateValue: "0", configValue: 2, wantChange: true, wantRequiresNew: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state := &terraform.InstanceState{
				ID: vhdPath,
				Attributes: map[string]string{
					"id":     vhdPath,
					"path":   vhdPath,
					"exists": "true",
					tc.key:   tc.stateValue,
				},
			}
			config := terraform.NewResourceConfigRaw(map[string]interface{}{
				"path": vhdPath,
				tc.key: tc.configValue,
			})

			diff, err := resourceHyperVVhd().Diff(context.Background(), state, config, nil)
			if err != nil {
				t.Fatalf("diff failed: %v", err)
			}

			var attr *terraform.ResourceAttrDiff
			if diff != nil {
				attr = diff.Attributes[tc.key]
			}

			if !tc.wantChange {
				if attr != nil {
					t.Fatalf("expected no %s diff, got %#v", tc.key, attr)
				}
				return
			}

			if attr == nil {
				t.Fatalf("expected a %s diff, got none", tc.key)
			}

			if attr.RequiresNew != tc.wantRequiresNew {
				t.Fatalf("%s RequiresNew = %v, want %v", tc.key, attr.RequiresNew, tc.wantRequiresNew)
			}
		})
	}
}
