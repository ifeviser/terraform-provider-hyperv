resource "hyperv_dda_assignment" "example" {
  vm_name            = hyperv_machine_instance.example.name
  resource_pool_name = "Auto-RTX-4090-B175-D0-F0"
}
