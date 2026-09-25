package hyperv

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"text/template"
)

func TestEncodeDdaAssignmentRequest(t *testing.T) {
	t.Parallel()

	request := ddaAssignmentRequest{
		VMName:           `vm'name`,
		ResourcePoolName: `pool"name`,
		LocationPath:     `PCIROOT(0)#PCI(0100)`,
	}
	encoded, err := encodeDdaAssignmentRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var got ddaAssignmentRequest
	if err := json.Unmarshal(decoded, &got); err != nil {
		t.Fatal(err)
	}
	if got != request {
		t.Fatalf("decoded request = %#v, want %#v", got, request)
	}
}

func TestDdaAssignmentTemplatesUseExactAssignmentIdentity(t *testing.T) {
	t.Parallel()

	request, err := encodeDdaAssignmentRequest(ddaAssignmentRequest{
		VMName:           "test-vm",
		ResourcePoolName: "test-pool",
		LocationPath:     "test-location",
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, script := range map[string]*template.Template{
		"get":    getDdaAssignmentTemplate,
		"create": createDdaAssignmentTemplate,
		"delete": deleteDdaAssignmentTemplate,
	} {
		var rendered bytes.Buffer
		if err := script.Execute(&rendered, ddaAssignmentArgs{Request: request}); err != nil {
			t.Fatalf("%s template: %v", name, err)
		}
		if strings.Contains(rendered.String(), "test-vm") || strings.Contains(rendered.String(), "test-pool") {
			t.Fatalf("%s template contains unencoded request values", name)
		}
		if strings.Contains(rendered.String(), "Get-VMAssignableDevice -VM $vm -ErrorAction SilentlyContinue") {
			t.Fatalf("%s template suppresses assignment query errors", name)
		}
		if strings.Contains(rendered.String(), "Get-VM -Name") {
			t.Fatalf("%s template uses wildcard-capable VM name lookup", name)
		}
	}

	var deleteScript bytes.Buffer
	if err := deleteDdaAssignmentTemplate.Execute(&deleteScript, ddaAssignmentArgs{Request: request}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(deleteScript.String(), "expected '$($request.LocationPath)'") {
		t.Fatal("delete template must guard the recorded location path")
	}
}
