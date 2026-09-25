package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/taliesins/terraform-provider-hyperv/api"
)

func TestDdaAssignmentSchema(t *testing.T) {
	t.Parallel()

	resource := resourceHyperVDdaAssignment()
	for _, name := range []string{"vm_name", "resource_pool_name"} {
		field := resource.Schema[name]
		if !field.Required || !field.ForceNew {
			t.Fatalf("%s must be required and ForceNew", name)
		}
	}
	for _, name := range []string{"location_path", "instance_path"} {
		if !resource.Schema[name].Computed {
			t.Fatalf("%s must be computed", name)
		}
	}
}

func TestParseDdaAssignmentID(t *testing.T) {
	t.Parallel()

	vmName, poolName, err := parseDdaAssignmentID("test-vm|test-pool")
	if err != nil {
		t.Fatal(err)
	}
	if vmName != "test-vm" || poolName != "test-pool" {
		t.Fatalf("unexpected identity %q %q", vmName, poolName)
	}
	for _, id := range []string{"", "vm", "vm|", "|pool", "vm|pool|extra"} {
		if _, _, err := parseDdaAssignmentID(id); err == nil {
			t.Fatalf("expected %q to be rejected", id)
		}
	}
}

func TestDdaAssignmentImport(t *testing.T) {
	t.Parallel()

	resource := resourceHyperVDdaAssignment()
	data := schema.TestResourceDataRaw(t, resource.Schema, nil)
	data.SetId("test-vm|test-pool")
	imported, err := resourceHyperVDdaAssignmentImport(context.Background(), data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported) != 1 || imported[0].Get("vm_name") != "test-vm" || imported[0].Get("resource_pool_name") != "test-pool" {
		t.Fatalf("unexpected imported state: %#v", imported)
	}
}

func TestSetDdaAssignmentComputedStatePreservesConfiguredIdentity(t *testing.T) {
	t.Parallel()

	resource := resourceHyperVDdaAssignment()
	data := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{
		"vm_name":            "test-vm",
		"resource_pool_name": "Test-Pool",
	})
	diagnostics := setDdaAssignmentComputedState(data, api.DdaAssignment{
		VMName:           "test-vm",
		ResourcePoolName: "test-pool",
		LocationPath:     "test-location",
		InstancePath:     "test-instance",
	})
	if diagnostics.HasError() {
		t.Fatalf("setting computed state: %v", diagnostics)
	}
	if got := data.Get("resource_pool_name"); got != "Test-Pool" {
		t.Fatalf("resource_pool_name = %q, want configured casing", got)
	}
	if got := data.Get("location_path"); got != "test-location" {
		t.Fatalf("location_path = %q", got)
	}
}
