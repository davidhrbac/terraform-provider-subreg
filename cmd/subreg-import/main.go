package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/davidhrbac/terraform-provider-subreg/internal/client"
)

func main() {
	var domain string
	flag.StringVar(&domain, "domain", "", "domain to import")
	flag.Parse()

	if domain == "" {
		domain = envFirst("TF_VAR_subreg_domain", "SUBREG_DOMAIN")
	}
	if domain == "" {
		fatal("missing domain (use -domain or TF_VAR_subreg_domain)")
	}

	login := envFirst("TF_VAR_subreg_login", "SUBREG_LOGIN")
	password := envFirst("TF_VAR_subreg_password", "SUBREG_PASSWORD")
	wsdlURL := envFirst("TF_VAR_subreg_wsdl_url", "SUBREG_WSDL_URL")
	if wsdlURL == "" {
		wsdlURL = "https://subreg.cz/wsdl"
	}

	if login == "" || password == "" {
		fatal("missing credentials (set TF_VAR_subreg_login and TF_VAR_subreg_password)")
	}

	api, err := client.New(login, password, wsdlURL)
	if err != nil {
		fatal(err.Error())
	}

	records, err := api.GetDNSZone(context.Background(), domain)
	if err != nil {
		fatal(err.Error())
	}

	if err := writeImports(os.Stdout, domain, records); err != nil {
		fatal(err.Error())
	}
}

func envFirst(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func writeImports(w io.Writer, domain string, records []client.DNSRecord) error {
	domainResourceName := resourceNameForDomain(domain)
	if _, err := fmt.Fprintf(w, "import {\n  to = subreg_domain.%s\n  id = \"%s\"\n}\n\n", domainResourceName, domain); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "import {\n  to = subreg_dns_zone.%s\n  id = \"%s\"\n}\n\n", domainResourceName, domain); err != nil {
		return err
	}

	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	used := map[string]int{}

	for _, record := range records {
		name := recordNamePart(record.Name)
		typePart := strings.ToLower(strings.TrimSpace(record.Type))
		if typePart == "" {
			typePart = "record"
		}
		base := fmt.Sprintf("%s_%s", name, typePart)
		base = sanitize(base)
		if base == "" {
			base = "record"
		}
		resourceName := fmt.Sprintf("%s_%d", base, record.ID)
		if count, ok := used[resourceName]; ok {
			count++
			used[resourceName] = count
			resourceName = fmt.Sprintf("%s_%d", resourceName, count)
		} else {
			used[resourceName] = 1
		}

		if _, err := fmt.Fprintf(w, "import {\n  to = subreg_dns_record.%s\n  id = \"%s:%d\"\n}\n\n", resourceName, domain, record.ID); err != nil {
			return err
		}
	}

	return nil
}

func recordNamePart(name string) string {
	value := strings.TrimSpace(name)
	if value == "" || value == "@" {
		return "root"
	}
	if value == "*" {
		return "wildcard"
	}
	return value
}

func resourceNameForDomain(domain string) string {
	name := sanitize(domain)
	if name == "" {
		return "domain"
	}
	return name
}

func sanitize(value string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	clean := strings.ToLower(value)
	clean = re.ReplaceAllString(clean, "_")
	clean = strings.Trim(clean, "_")
	clean = strings.ReplaceAll(clean, "__", "_")
	return clean
}
