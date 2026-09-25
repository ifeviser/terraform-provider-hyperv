package hyperv

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/taliesins/terraform-provider-hyperv/api"
)

type ddaAssignmentArgs struct {
	Request string
}

type ddaAssignmentRequest struct {
	VMName           string
	ResourcePoolName string
	LocationPath     string
}

var getDdaAssignmentTemplate = template.Must(template.New("GetDdaAssignment").Parse(`
$ErrorActionPreference = 'Stop'
Import-Module Hyper-V
$request = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('{{.Request}}')) | ConvertFrom-Json
$vm = Get-VM -ErrorAction Stop |
    Where-Object { [StringComparer]::OrdinalIgnoreCase.Equals([string]$_.Name, [string]$request.VMName) }
if (-not $vm) {
    @{ Exists = $false } | ConvertTo-Json -Compress
    exit 0
}
$matches = @(Get-VMAssignableDevice -VM $vm -ErrorAction Stop |
    Where-Object { [StringComparer]::OrdinalIgnoreCase.Equals([string]$_.ResourcePoolName, [string]$request.ResourcePoolName) })
if ($matches.Count -gt 1) {
    throw "VM '$($request.VMName)' has multiple devices assigned from resource pool '$($request.ResourcePoolName)'"
}
if ($matches.Count -eq 0) {
    @{ Exists = $false; VMName = [string]$request.VMName; ResourcePoolName = [string]$request.ResourcePoolName } | ConvertTo-Json -Compress
    exit 0
}
$device = $matches[0]
@{
    Exists = $true
    VMName = [string]$request.VMName
    ResourcePoolName = [string]$device.ResourcePoolName
    LocationPath = [string]$device.LocationPath
    InstancePath = [string]$device.InstancePath
} | ConvertTo-Json -Compress
`))

var createDdaAssignmentTemplate = template.Must(template.New("CreateDdaAssignment").Parse(`
$ErrorActionPreference = 'Stop'
Import-Module Hyper-V
$request = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('{{.Request}}')) | ConvertFrom-Json
$vm = Get-VM -ErrorAction Stop |
    Where-Object { [StringComparer]::OrdinalIgnoreCase.Equals([string]$_.Name, [string]$request.VMName) }
if (-not $vm) {
    throw "VM '$($request.VMName)' does not exist"
}
if ($vm.State -ne [Microsoft.HyperV.PowerShell.VMState]::Off) {
    throw "VM '$($request.VMName)' must be Off before assigning a DDA device; current state is '$($vm.State)'"
}
$pool = Get-VMResourcePool -Name ([string]$request.ResourcePoolName) -ResourcePoolType PciExpress -ErrorAction SilentlyContinue
if (-not $pool) {
    throw "PCI Express resource pool '$($request.ResourcePoolName)' does not exist"
}
$assignments = @(Get-VM -ErrorAction Stop |
    Get-VMAssignableDevice -ErrorAction Stop |
    Where-Object { [StringComparer]::OrdinalIgnoreCase.Equals([string]$_.ResourcePoolName, [string]$request.ResourcePoolName) })
if ($assignments.Count -gt 0) {
    $owners = @($assignments | ForEach-Object { [string]$_.VMName } | Select-Object -Unique) -join ', '
    throw "PCI Express resource pool '$($request.ResourcePoolName)' is already assigned to VM(s): $owners"
}
$poolDevices = @(Get-VMHostAssignableDevice -ResourcePoolName ([string]$request.ResourcePoolName) -ErrorAction Stop)
if ($poolDevices.Count -ne 1) {
    throw "PCI Express resource pool '$($request.ResourcePoolName)' must contain exactly one host-assignable device; found $($poolDevices.Count)"
}
$device = Add-VMAssignableDevice -VM $vm -ResourcePoolName ([string]$request.ResourcePoolName) -Passthru -ErrorAction Stop
@{
    Exists = $true
    VMName = [string]$request.VMName
    ResourcePoolName = [string]$device.ResourcePoolName
    LocationPath = [string]$device.LocationPath
    InstancePath = [string]$device.InstancePath
} | ConvertTo-Json -Compress
`))

var deleteDdaAssignmentTemplate = template.Must(template.New("DeleteDdaAssignment").Parse(`
$ErrorActionPreference = 'Stop'
Import-Module Hyper-V
$request = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('{{.Request}}')) | ConvertFrom-Json
$vm = Get-VM -ErrorAction Stop |
    Where-Object { [StringComparer]::OrdinalIgnoreCase.Equals([string]$_.Name, [string]$request.VMName) }
if (-not $vm) {
    @{ Deleted = $true } | ConvertTo-Json -Compress
    exit 0
}
$matches = @(Get-VMAssignableDevice -VM $vm -ErrorAction Stop |
    Where-Object { [StringComparer]::OrdinalIgnoreCase.Equals([string]$_.ResourcePoolName, [string]$request.ResourcePoolName) })
if ($matches.Count -eq 0) {
    @{ Deleted = $true } | ConvertTo-Json -Compress
    exit 0
}
if ($matches.Count -gt 1) {
    throw "Refusing to remove DDA assignment: VM '$($request.VMName)' has multiple devices from pool '$($request.ResourcePoolName)'"
}
$device = $matches[0]
if ($request.LocationPath -and
    -not [StringComparer]::OrdinalIgnoreCase.Equals([string]$device.LocationPath, [string]$request.LocationPath)) {
    throw "Refusing to remove DDA assignment: pool '$($request.ResourcePoolName)' is assigned at '$($device.LocationPath)', expected '$($request.LocationPath)'"
}
$device | Remove-VMAssignableDevice -Confirm:$false -ErrorAction Stop
@{ Deleted = $true } | ConvertTo-Json -Compress
`))

func encodeDdaAssignmentRequest(request ddaAssignmentRequest) (string, error) {
	value, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encoding DDA assignment request: %w", err)
	}
	return base64.StdEncoding.EncodeToString(value), nil
}

func (c *ClientConfig) GetDdaAssignment(ctx context.Context, vmName, resourcePoolName string) (result api.DdaAssignment, err error) {
	request, err := encodeDdaAssignmentRequest(ddaAssignmentRequest{
		VMName:           vmName,
		ResourcePoolName: resourcePoolName,
	})
	if err != nil {
		return result, err
	}
	err = c.ScriptRunner.RunScriptWithResult(ctx, getDdaAssignmentTemplate, ddaAssignmentArgs{Request: request}, &result)
	return result, err
}

func (c *ClientConfig) CreateDdaAssignment(ctx context.Context, vmName, resourcePoolName string) (result api.DdaAssignment, err error) {
	request, err := encodeDdaAssignmentRequest(ddaAssignmentRequest{
		VMName:           vmName,
		ResourcePoolName: resourcePoolName,
	})
	if err != nil {
		return result, err
	}
	err = c.ScriptRunner.RunScriptWithResult(ctx, createDdaAssignmentTemplate, ddaAssignmentArgs{Request: request}, &result)
	return result, err
}

func (c *ClientConfig) DeleteDdaAssignment(ctx context.Context, vmName, resourcePoolName, locationPath string) error {
	request, err := encodeDdaAssignmentRequest(ddaAssignmentRequest{
		VMName:           vmName,
		ResourcePoolName: resourcePoolName,
		LocationPath:     locationPath,
	})
	if err != nil {
		return err
	}
	var result struct {
		Deleted bool
	}
	if err := c.ScriptRunner.RunScriptWithResult(ctx, deleteDdaAssignmentTemplate, ddaAssignmentArgs{Request: request}, &result); err != nil {
		return err
	}
	if !result.Deleted {
		return fmt.Errorf("removing DDA assignment from VM %q did not report success", vmName)
	}
	return nil
}
