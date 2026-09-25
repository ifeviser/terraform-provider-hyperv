//nolint:forcetypeassert // Resource schema guarantees value types retrieved from Terraform state.
package provider

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/taliesins/terraform-provider-hyperv/api"
)

const (
	createDdaAssignmentTimeout = 2 * time.Minute
	readDdaAssignmentTimeout   = 1 * time.Minute
	deleteDdaAssignmentTimeout = 2 * time.Minute
)

func resourceHyperVDdaAssignment() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages one Hyper-V Discrete Device Assignment from an existing PCI Express resource pool to a virtual machine.",
		CreateContext: resourceHyperVDdaAssignmentCreate,
		ReadContext:   resourceHyperVDdaAssignmentRead,
		DeleteContext: resourceHyperVDdaAssignmentDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceHyperVDdaAssignmentImport,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(createDdaAssignmentTimeout),
			Read:   schema.DefaultTimeout(readDdaAssignmentTimeout),
			Delete: schema.DefaultTimeout(deleteDdaAssignmentTimeout),
		},
		Schema: map[string]*schema.Schema{
			"vm_name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateDdaIdentityPart,
				Description:  "Name of the virtual machine that receives the device. The VM must exist on this provider's host and be powered off.",
			},
			"resource_pool_name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateDdaIdentityPart,
				Description:  "Name of an existing PCI Express resource pool containing exactly one host-assignable device.",
			},
			"location_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Physical location path selected by Hyper-V from the resource pool.",
			},
			"instance_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Device instance path reported by Hyper-V.",
			},
		},
	}
}

func validateDdaIdentityPart(value interface{}, key string) ([]string, []error) {
	text := strings.TrimSpace(value.(string))
	if text == "" {
		return nil, []error{fmt.Errorf("%s must not be empty", key)}
	}
	if strings.Contains(text, "|") {
		return nil, []error{fmt.Errorf("%s must not contain |", key)}
	}
	return nil, nil
}

func ddaAssignmentID(vmName, resourcePoolName string) string {
	return vmName + "|" + resourcePoolName
}

func parseDdaAssignmentID(id string) (string, string, error) {
	parts := strings.Split(id, "|")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("DDA assignment ID must be <vm_name>|<resource_pool_name>")
	}
	return parts[0], parts[1], nil
}

func resourceHyperVDdaAssignmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(api.Client)
	vmName := d.Get("vm_name").(string)
	resourcePoolName := d.Get("resource_pool_name").(string)

	existing, err := client.GetDdaAssignment(ctx, vmName, resourcePoolName)
	if err != nil {
		return diag.FromErr(fmt.Errorf("checking DDA assignment for VM %q from pool %q: %w", vmName, resourcePoolName, err))
	}
	if existing.Exists {
		return diag.Errorf("DDA assignment from pool %q already exists on VM %q; import it with: terraform import hyperv_dda_assignment.<name> %q", resourcePoolName, vmName, ddaAssignmentID(vmName, resourcePoolName))
	}

	assignment, err := client.CreateDdaAssignment(ctx, vmName, resourcePoolName)
	if err != nil {
		return diag.FromErr(fmt.Errorf("assigning DDA pool %q to VM %q: %w", resourcePoolName, vmName, err))
	}
	if !assignment.Exists {
		return diag.Errorf("assigning DDA pool %q to VM %q did not return an assignment", resourcePoolName, vmName)
	}
	d.SetId(ddaAssignmentID(vmName, resourcePoolName))
	return setDdaAssignmentComputedState(d, assignment)
}

func resourceHyperVDdaAssignmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(api.Client)
	vmName := d.Get("vm_name").(string)
	resourcePoolName := d.Get("resource_pool_name").(string)
	if vmName == "" || resourcePoolName == "" {
		importedVM, importedPool, err := parseDdaAssignmentID(d.Id())
		if err != nil {
			return diag.FromErr(err)
		}
		vmName, resourcePoolName = importedVM, importedPool
	}

	assignment, err := client.GetDdaAssignment(ctx, vmName, resourcePoolName)
	if err != nil {
		return diag.FromErr(fmt.Errorf("reading DDA assignment for VM %q from pool %q: %w", vmName, resourcePoolName, err))
	}
	if !assignment.Exists {
		log.Printf("[INFO][hyperv][read] DDA assignment %q is absent", d.Id())
		d.SetId("")
		return nil
	}
	return setDdaAssignmentComputedState(d, assignment)
}

func setDdaAssignmentComputedState(d *schema.ResourceData, assignment api.DdaAssignment) diag.Diagnostics {
	for key, value := range map[string]interface{}{
		"location_path": assignment.LocationPath,
		"instance_path": assignment.InstancePath,
	} {
		if err := d.Set(key, value); err != nil {
			return diag.FromErr(fmt.Errorf("setting %s for DDA assignment: %w", key, err))
		}
	}
	return nil
}

func resourceHyperVDdaAssignmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(api.Client)
	vmName := d.Get("vm_name").(string)
	resourcePoolName := d.Get("resource_pool_name").(string)
	locationPath := d.Get("location_path").(string)

	if err := client.DeleteDdaAssignment(ctx, vmName, resourcePoolName, locationPath); err != nil {
		return diag.FromErr(fmt.Errorf("removing DDA pool %q from VM %q: %w", resourcePoolName, vmName, err))
	}
	d.SetId("")
	return nil
}

func resourceHyperVDdaAssignmentImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	vmName, resourcePoolName, err := parseDdaAssignmentID(d.Id())
	if err != nil {
		return nil, err
	}
	if err := d.Set("vm_name", vmName); err != nil {
		return nil, err
	}
	if err := d.Set("resource_pool_name", resourcePoolName); err != nil {
		return nil, err
	}
	d.SetId(ddaAssignmentID(vmName, resourcePoolName))
	return []*schema.ResourceData{d}, nil
}
