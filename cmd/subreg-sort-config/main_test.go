package main

import (
	"strings"
	"testing"
)

func TestSortGeneratedConfigOrdersDomainZoneRecords(t *testing.T) {
	input := strings.Join([]string{
		"# __generated__ by Terraform",
		"# Please review these resources and move them into your main configuration files.",
		"",
		"# __generated__ by Terraform from \"example.com:3\"",
		"resource \"subreg_dns_record\" \"third\" {",
		"  domain = \"example.com\"",
		"}",
		"",
		"# __generated__ by Terraform from \"example.com\"",
		"resource \"subreg_dns_zone\" \"zone\" {",
		"  domain = \"example.com\"",
		"  dnssec = true",
		"}",
		"",
		"# __generated__ by Terraform from \"example.com\"",
		"resource \"subreg_domain\" \"domain\" {",
		"  autorenew = true",
		"  domain = \"example.com\"",
		"}",
		"",
		"# __generated__ by Terraform from \"example.com:1\"",
		"resource \"subreg_dns_record\" \"first\" {",
		"  domain = \"example.com\"",
		"}",
	}, "\n")

	output, err := sortGeneratedConfig(input)
	if err != nil {
		t.Fatalf("sortGeneratedConfig failed: %v", err)
	}

	firstDomain := strings.Index(output, "subreg_domain")
	firstZone := strings.Index(output, "subreg_dns_zone")
	firstRecord := strings.Index(output, "subreg_dns_record")
	if !(firstDomain < firstZone && firstZone < firstRecord) {
		t.Fatalf("unexpected order:\n%s", output)
	}
	if strings.Index(output, "first") > strings.Index(output, "third") {
		t.Fatalf("expected record blocks to be sorted by import id:\n%s", output)
	}
}
