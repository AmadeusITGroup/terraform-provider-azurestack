package dns

//go:generate go run ../../tools/generator-resource-id/main.go -path=./ -name=DnsZone -id=/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/resGroup1/providers/Microsoft.Network/dnszones/zone1
//go:generate go run ../../tools/generator-resource-id/main.go -path=./ -name=ARecord -id=/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/resGroup1/providers/Microsoft.Network/dnszones/zone1/A/eh1
