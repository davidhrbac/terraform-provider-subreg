package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCallRespectsContextCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)

	client := &Client{
		httpClient:      server.Client(),
		endpointURL:     server.URL,
		targetNamespace: "http://subreg.cz/types",
		wsdl: &wsdlDefinitions{
			Bindings: []*wsdlBinding{{
				Operations: []*wsdlOperation{{
					Name:           "Login",
					SoapOperations: []*soapOperation{{SoapAction: "http://subreg.cz/wsdl#Login"}},
				}},
			}},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := client.call(ctx, "Login", map[string]any{
			"login":    "user",
			"password": "secret",
		})
		errCh <- err
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}

	cancel()

	err := <-errCh
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMatchCreatedDNSRecordID(t *testing.T) {
	records := []DNSRecord{{
		ID:      42,
		Name:    "@",
		Type:    "A",
		Content: "203.0.113.10",
	}}

	id, err := matchCreatedDNSRecordID(records, DNSRecordInput{
		Name:    "",
		Type:    "a",
		Content: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("matchCreatedDNSRecordID returned error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id 42, got %d", id)
	}
}

func TestMatchCreatedDNSRecordIDRejectsDuplicates(t *testing.T) {
	records := []DNSRecord{{
		ID:      41,
		Name:    "www",
		Type:    "A",
		Content: "203.0.113.11",
	}, {
		ID:      42,
		Name:    "www",
		Type:    "A",
		Content: "203.0.113.11",
	}}

	_, err := matchCreatedDNSRecordID(records, DNSRecordInput{
		Name:    "www",
		Type:    "A",
		Content: "203.0.113.11",
	})
	if err == nil {
		t.Fatal("expected duplicate match error")
	}
	if !strings.Contains(err.Error(), "multiple DNS records matched") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeDNSZoneRecords(t *testing.T) {
	records, err := decodeDNSZoneRecords([]byte(`<domain>example.com</domain><records></records>`))
	if err != nil {
		t.Fatalf("expected empty zone payload to parse, got error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no records, got %d", len(records))
	}

	_, err = decodeDNSZoneRecords([]byte(`<unexpected><value>1</value></unexpected>`))
	if err == nil {
		t.Fatal("expected malformed payload to fail")
	}
}

func TestGetWsdlDefinitionsAndNamespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<definitions name="SubregCz"
	targetNamespace="http://subreg.cz/wsdl"
	xmlns:tn="http://subreg.cz/wsdl"
	xmlns:ns="http://subreg.cz/types"
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
	xmlns="http://schemas.xmlsoap.org/wsdl/">
	<types>
		<xs:schema targetNamespace="http://subreg.cz/types" xmlns="http://www.w3.org/2001/XMLSchema"></xs:schema>
	</types>
	<binding name="SubregCzBinding" type="tn:SubregCz">
		<soap:binding style="document" transport="http://schemas.xmlsoap.org/soap/http" />
		<operation name="Login">
			<soap:operation soapAction="http://subreg.cz/wsdl#Login" />
		</operation>
	</binding>
	<service name="SubregCzService">
		<port name="SubregCz" binding="tn:SubregCzBinding">
			<soap:address location="https://subreg.example/soap/cmd.php?soap_format=1" />
		</port>
	</service>
</definitions>`))
	}))
	defer server.Close()

	defs, err := getWsdlDefinitions(server.URL, server.Client())
	if err != nil {
		t.Fatalf("getWsdlDefinitions failed: %v", err)
	}

	endpoint, namespace, err := wsdlEndpointAndNamespace(defs)
	if err != nil {
		t.Fatalf("wsdlEndpointAndNamespace failed: %v", err)
	}
	if endpoint != "https://subreg.example/soap/cmd.php?soap_format=1" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
	if namespace != "http://subreg.cz/types" {
		t.Fatalf("unexpected namespace: %s", namespace)
	}
	if got := defs.GetSoapActionFromWsdlOperation("Login"); got != "http://subreg.cz/wsdl#Login" {
		t.Fatalf("unexpected soap action: %s", got)
	}
}

func TestCallParsesSOAPEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <Login_Container>
      <response>
        <status>ok</status>
        <data><ssid>abc123</ssid></data>
      </response>
    </Login_Container>
  </soap:Body>
</soap:Envelope>`))
	}))
	defer server.Close()

	client := &Client{
		httpClient:      server.Client(),
		endpointURL:     server.URL,
		targetNamespace: "http://subreg.cz/types",
		wsdl: &wsdlDefinitions{
			Bindings: []*wsdlBinding{{
				Operations: []*wsdlOperation{{
					Name:           "Login",
					SoapOperations: []*soapOperation{{SoapAction: "http://subreg.cz/wsdl#Login"}},
				}},
			}},
		},
	}

	resp, err := client.call(context.Background(), "Login", map[string]any{
		"login":    "user",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if got := strings.TrimSpace(string(resp.Data.InnerXML)); !strings.Contains(got, "abc123") {
		t.Fatalf("unexpected inner xml: %q", got)
	}
}
