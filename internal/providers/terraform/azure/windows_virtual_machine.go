package azure

import (
	"github.com/infracost/infracost/internal/resources/azure"
	"github.com/infracost/infracost/internal/schema"
)

func getWindowsVirtualMachineRegistryItem() *schema.RegistryItem {
	return &schema.RegistryItem{
		Name:  "azurerm_windows_virtual_machine",
		RFunc: NewWindowsVirtualMachine,
		Notes: []string{
			"Low priority, Spot and Reserved instances are not supported.",
		},
	}
}

func NewWindowsVirtualMachine(d *schema.ResourceData, u *schema.UsageData) *schema.Resource {
	r := &azure.WindowsVirtualMachine{
		Address:                               d.Address,
		Region:                                lookupRegion(d, []string{}),
		Size:                                  d.Get("size").String(),
		LicenseType:                           d.Get("license_type").String(),
		AdditionalCapabilitiesUltraSSDEnabled: d.Get("additional_capabilities.0.ultra_ssd_enabled").Bool(),
	}
	if len(d.Get("os_disk").Array()) > 0 {
		diskData := d.Get("os_disk").Array()[0]
		r.OSDiskData = &azure.ManagedDiskData{
			DiskType:   diskData.Get("storage_account_type").String(),
			DiskSizeGB: diskData.Get("disk_size_gb").Int(),
		}
	}
	r.PopulateUsage(u)
	return r.BuildResource()
}
