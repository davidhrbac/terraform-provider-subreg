package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/davidhrbac/terraform-provider-subreg/internal/client"
)

func TestWriteImportsIncludesDomainResources(t *testing.T) {
	var buf bytes.Buffer
	err := writeImports(&buf, "example.com", []client.DNSRecord{{
		ID:   2,
		Name: "www",
		Type: "A",
	}, {
		ID:   1,
		Name: "@",
		Type: "MX",
	}})
	if err != nil {
		t.Fatalf("writeImports failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"to = subreg_domain.example_com",
		"to = subreg_dns_zone.example_com",
		"to = subreg_dns_record.root_mx_1",
		"to = subreg_dns_record.www_a_2",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}

	if strings.Index(output, "subreg_domain.example_com") > strings.Index(output, "subreg_dns_zone.example_com") {
		t.Fatal("expected domain import before dns zone import")
	}
	if strings.Index(output, "subreg_dns_zone.example_com") > strings.Index(output, "subreg_dns_record.root_mx_1") {
		t.Fatal("expected dns zone import before record imports")
	}
}

func TestWriteImportsAllowsEmptyZone(t *testing.T) {
	var buf bytes.Buffer
	if err := writeImports(&buf, "example.com", nil); err != nil {
		t.Fatalf("writeImports failed: %v", err)
	}

	output := buf.String()
	if strings.Count(output, "import {") != 2 {
		t.Fatalf("expected 2 import blocks, got output:\n%s", output)
	}
}
