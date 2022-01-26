---
layout: "azurestack"
page_title: "Azure Resource Manager: azurestack_dns_zone"
sidebar_current: "docs-azurestack-datasource-dns-zone"
description: |-
  Get information about the configuration of the azurestack provider.
---

# Data Source: azurestack_dns_zone

Use this data source to access the configuration of the Azure Stack
provider.

## Example Usage

```hcl
data "azurestack_dns_zone" "zone" {
  name                = "testzone.azure.com"
  resource_group_name = "networking"
}

output "dns_zone_id" {
  value = "${data.azurestack_dns_zone.zone.id}"
}
```

## Argument Reference

* `name` - (Required) Specifies the name of the DNS zone.
* `resource_group_name` - (Required) Specifies the name of the resource group the DNS zone is located in.

## Attributes Reference

* `id` - The ID of the DNS Zone.

* `number_of_record_sets` The number of records already in the zone.
* `max_number_of_record_sets` Maximum number of Records in the zone.
* `name_servers` - A list of values that make up the NS record for the zone.
* `tags` - A mapping of tags to assign to the EventHub Namespace.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://www.terraform.io/docs/configuration/resources.html#timeouts) for certain actions:

* `read` - (Defaults to 5 minutes) Used when retrieving the DNS Zone.