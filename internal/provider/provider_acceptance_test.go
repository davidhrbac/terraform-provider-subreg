package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/davidhrbac/terraform-provider-subreg/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccDNSRecordResource(t *testing.T) {
	if !acceptanceEnabled() {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	domain := acceptanceDomain(t)
	name := acctest.RandomWithPrefix("tf-acc")
	initialContent := acctest.RandomWithPrefix("initial") + ".example.invalid"
	updatedContent := acctest.RandomWithPrefix("updated") + ".example.invalid"

	resource.Test(t, resource.TestCase{
		PreCheck:                  func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories:  acceptanceProviderFactories(),
		CheckDestroy:              acceptanceCheckDNSRecordDestroy,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: acceptanceDNSRecordConfig(domain, name, initialContent),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("subreg_dns_record.test", "domain", domain),
					resource.TestCheckResourceAttr("subreg_dns_record.test", "name", name),
					resource.TestCheckResourceAttr("subreg_dns_record.test", "type", "TXT"),
					resource.TestCheckResourceAttr("subreg_dns_record.test", "content", initialContent),
					resource.TestCheckResourceAttr("subreg_dns_record.test", "ttl", "600"),
				),
			},
			{
				Config: acceptanceDNSRecordConfig(domain, name, updatedContent),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("subreg_dns_record.test", "content", updatedContent),
				),
			},
			{
				ResourceName:      "subreg_dns_record.test",
				ImportStateIdFunc: acceptanceImportStateIDFunc,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccDNSRootRecordResource(t *testing.T) {
	if !acceptanceEnabled() {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	domain := acceptanceDomain(t)
	content := acctest.RandomWithPrefix("root") + ".example.invalid"

	resource.Test(t, resource.TestCase{
		PreCheck:                  func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories:  acceptanceProviderFactories(),
		CheckDestroy:              acceptanceCheckDNSRecordDestroy,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: acceptanceRootDNSRecordConfig(domain, content),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("subreg_dns_record.test", "domain", domain),
					resource.TestCheckResourceAttr("subreg_dns_record.test", "name", "@"),
					resource.TestCheckResourceAttr("subreg_dns_record.test", "type", "TXT"),
					resource.TestCheckResourceAttr("subreg_dns_record.test", "content", content),
					resource.TestCheckResourceAttr("subreg_dns_record.test", "ttl", "600"),
				),
			},
			{
				ResourceName:      "subreg_dns_record.test",
				ImportStateIdFunc: acceptanceImportStateIDFunc,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccDNSZoneDataSource(t *testing.T) {
	if !acceptanceEnabled() {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	domain := acceptanceDomain(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: acceptanceProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acceptanceDNSZoneConfig(domain),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.subreg_dns_zone.test", "domain", domain),
					acceptanceCheckResourceAttrIntGreaterThan("data.subreg_dns_zone.test", "records.#", 0),
				),
			},
		},
	})
}

func acceptanceEnabled() bool {
	return os.Getenv("TF_ACC") == "1"
}

func acceptancePreCheck(t *testing.T) {
	t.Helper()

	if os.Getenv("SUBREG_LOGIN") == "" {
		t.Fatal("set SUBREG_LOGIN")
	}
	if os.Getenv("SUBREG_PASSWORD") == "" {
		t.Fatal("set SUBREG_PASSWORD")
	}
	if acceptanceDomain(t) == "" {
		t.Fatal("set SUBREG_DOMAIN or TF_VAR_subreg_domain")
	}
}

func acceptanceDomain(t *testing.T) string {
	t.Helper()

	if value := strings.TrimSpace(os.Getenv("SUBREG_DOMAIN")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("TF_VAR_subreg_domain"))
}

func acceptanceProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"subreg": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func acceptanceProviderConfig() string {
	login := os.Getenv("SUBREG_LOGIN")
	password := os.Getenv("SUBREG_PASSWORD")
	wsdlURL := os.Getenv("SUBREG_WSDL_URL")
	if wsdlURL == "" {
		wsdlURL = defaultWSDLURL
	}

	return fmt.Sprintf(`
provider "subreg" {
  login    = %q
  password = %q
  wsdl_url = %q
}
`, login, password, wsdlURL)
}

func acceptanceDNSRecordConfig(domain, name, content string) string {
	return acceptanceProviderConfig() + fmt.Sprintf(`
resource "subreg_dns_record" "test" {
  domain  = %q
  name    = %q
  type    = "TXT"
  content = %q
  ttl     = 600
}
`, domain, name, content)
}

func acceptanceRootDNSRecordConfig(domain, content string) string {
	return acceptanceProviderConfig() + fmt.Sprintf(`
resource "subreg_dns_record" "test" {
  domain  = %q
  name    = "@"
  type    = "TXT"
  content = %q
  ttl     = 600
}
`, domain, content)
}

func acceptanceDNSZoneConfig(domain string) string {
	return acceptanceProviderConfig() + fmt.Sprintf(`
data "subreg_dns_zone" "test" {
  domain = %q
}
`, domain)
}

func acceptanceCheckDNSRecordDestroy(s *terraform.State) error {
	cli, err := client.New(os.Getenv("SUBREG_LOGIN"), os.Getenv("SUBREG_PASSWORD"), acceptanceWSDLURL())
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "subreg_dns_record" {
			continue
		}

		id, err := strconv.Atoi(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("invalid record id %q: %w", rs.Primary.ID, err)
		}
		domain := rs.Primary.Attributes["domain"]
		if domain == "" {
			return fmt.Errorf("missing domain for resource type %s", rs.Type)
		}

		_, found, err := cli.GetDNSRecordByID(context.Background(), domain, id)
		if err != nil {
			return err
		}
		if found {
			return fmt.Errorf("dns record still exists after destroy: type=%s id=%s", rs.Type, rs.Primary.ID)
		}
	}

	return nil
}

func acceptanceCheckResourceAttrIntGreaterThan(resourceName, attributeName string, min int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("missing resource %s", resourceName)
		}

		value, ok := rs.Primary.Attributes[attributeName]
		if !ok {
			return fmt.Errorf("missing attribute %s on %s", attributeName, resourceName)
		}

		count, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer %q for %s.%s: %w", value, resourceName, attributeName, err)
		}
		if count <= min {
			return fmt.Errorf("expected %s.%s to be > %d, got %d", resourceName, attributeName, min, count)
		}

		return nil
	}
}

func acceptanceImportStateIDFunc(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["subreg_dns_record.test"]
	if !ok {
		return "", fmt.Errorf("missing resource subreg_dns_record.test")
	}

	domain := rs.Primary.Attributes["domain"]
	if domain == "" {
		return "", fmt.Errorf("missing domain for import")
	}
	if rs.Primary.ID == "" {
		return "", fmt.Errorf("missing id for import")
	}

	return fmt.Sprintf("%s:%s", domain, rs.Primary.ID), nil
}

func acceptanceWSDLURL() string {
	if value := strings.TrimSpace(os.Getenv("SUBREG_WSDL_URL")); value != "" {
		return value
	}
	return "https://demoreg.net/wsdl"
}
