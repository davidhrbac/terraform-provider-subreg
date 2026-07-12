package client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAccDNSZoneSmoke(t *testing.T) {
	if !acceptanceEnabled() {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	cli := mustAcceptanceClient(t)
	records, err := cli.GetDNSZone(context.Background(), acceptanceDomain())
	if err != nil {
		t.Fatalf("GetDNSZone failed: %v", err)
	}
	if records == nil {
		t.Fatal("expected records slice, got nil")
	}
}

func TestAccDNSRecordCRUD(t *testing.T) {
	if !acceptanceEnabled() {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	cli := mustAcceptanceClient(t)
	domain := acceptanceDomain()
	name := acceptanceUniqueLabel("tf-acc")
	initialContent := acceptanceUniqueLabel("initial") + ".example.invalid"
	updatedContent := acceptanceUniqueLabel("updated") + ".example.invalid"

	recordID, err := cli.AddDNSRecordWithID(context.Background(), domain, DNSRecordInput{
		Name:    name,
		Type:    "TXT",
		Content: initialContent,
		TTL:     intPtr(600),
	})
	if err != nil {
		t.Fatalf("AddDNSRecordWithID failed: %v", err)
	}
	t.Cleanup(func() {
		_ = cli.DeleteDNSRecord(context.Background(), domain, recordID)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	record, err := waitForRecord(ctx, cli, domain, recordID)
	if err != nil {
		t.Fatalf("created record not readable: %v", err)
	}
	if record.Content != initialContent {
		t.Fatalf("unexpected created content: %q", record.Content)
	}

	if err := cli.ModifyDNSRecord(context.Background(), domain, recordID, DNSRecordInput{
		Name:    name,
		Type:    "TXT",
		Content: updatedContent,
		TTL:     intPtr(600),
	}); err != nil {
		t.Fatalf("ModifyDNSRecord failed: %v", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	record, err = waitForRecord(ctx, cli, domain, recordID)
	if err != nil {
		t.Fatalf("updated record not readable: %v", err)
	}
	if record.Content != updatedContent {
		t.Fatalf("unexpected updated content: %q", record.Content)
	}

	if err := cli.DeleteDNSRecord(context.Background(), domain, recordID); err != nil {
		t.Fatalf("DeleteDNSRecord failed: %v", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	if _, found, err := cli.GetDNSRecordByID(ctx, domain, recordID); err != nil {
		t.Fatalf("GetDNSRecordByID after delete failed: %v", err)
	} else if found {
		t.Fatal("record still found after delete")
	}
}

func TestAccDNSRootRecordCRUD(t *testing.T) {
	if !acceptanceEnabled() {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	cli := mustAcceptanceClient(t)
	domain := acceptanceDomain()
	content := acceptanceUniqueLabel("root") + ".example.invalid"

	recordID, err := cli.AddDNSRecordWithID(context.Background(), domain, DNSRecordInput{
		Name:    "@",
		Type:    "TXT",
		Content: content,
		TTL:     intPtr(600),
	})
	if err != nil {
		t.Fatalf("AddDNSRecordWithID failed: %v", err)
	}
	t.Cleanup(func() {
		_ = cli.DeleteDNSRecord(context.Background(), domain, recordID)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	record, err := waitForRecord(ctx, cli, domain, recordID)
	if err != nil {
		t.Fatalf("root record not readable: %v", err)
	}
	if normalizeRecordNameForMatch(record.Name) != "@" {
		t.Fatalf("unexpected root record name: %q", record.Name)
	}
	if record.Content != content {
		t.Fatalf("unexpected root record content: %q", record.Content)
	}

	if err := cli.DeleteDNSRecord(context.Background(), domain, recordID); err != nil {
		t.Fatalf("DeleteDNSRecord failed: %v", err)
	}
}

func acceptanceEnabled() bool {
	return os.Getenv("TF_ACC") == "1"
}

func mustAcceptanceClient(t *testing.T) *Client {
	t.Helper()

	login := os.Getenv("SUBREG_LOGIN")
	password := os.Getenv("SUBREG_PASSWORD")
	wsdlURL := os.Getenv("SUBREG_WSDL_URL")
	if wsdlURL == "" {
		wsdlURL = "https://demoreg.net/wsdl"
	}

	if login == "" || password == "" {
		t.Fatal("set SUBREG_LOGIN and SUBREG_PASSWORD")
	}
	if domain := acceptanceDomain(); domain == "" {
		t.Fatal("set SUBREG_DOMAIN or TF_VAR_subreg_domain")
	}

	cli, err := New(login, password, wsdlURL)
	if err != nil {
		t.Fatalf("New client failed: %v", err)
	}

	return cli
}

func acceptanceDomain() string {
	if value := strings.TrimSpace(os.Getenv("SUBREG_DOMAIN")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("TF_VAR_subreg_domain"))
}

func acceptanceUniqueLabel(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func waitForRecord(ctx context.Context, cli *Client, domain string, id int) (DNSRecord, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		record, found, err := cli.GetDNSRecordByID(ctx, domain, id)
		if err != nil {
			return DNSRecord{}, err
		}
		if found {
			return record, nil
		}

		select {
		case <-ctx.Done():
			return DNSRecord{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func intPtr(v int) *int { return &v }
