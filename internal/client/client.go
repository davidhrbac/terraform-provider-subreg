package client

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html/charset"
)

const defaultTimeout = 30 * time.Second

const soapPrefix = "soap"

type Client struct {
	httpClient      *http.Client
	endpointURL     string
	targetNamespace string
	wsdl            *wsdlDefinitions
	login           string
	password        string

	mu   sync.Mutex
	ssid string
}

type responseEnvelope struct {
	Response responseBody `xml:"response"`
}

type soapEnvelope struct {
	Body soapBody `xml:"Body"`
}

type soapBody struct {
	InnerXML []byte `xml:",innerxml"`
}

type responseBody struct {
	Status string         `xml:"status"`
	Data   responseData   `xml:"data"`
	Error  *responseError `xml:"error"`
}

type responseData struct {
	InnerXML []byte `xml:",innerxml"`
}

type responseError struct {
	ErrorCode *responseErrorCode `xml:"errorcode"`
	ErrorMsg  string             `xml:"errormsg"`
}

type responseErrorCode struct {
	Major int `xml:"major"`
	Minor int `xml:"minor"`
}

type wsdlDefinitions struct {
	Name            string         `xml:"name,attr"`
	TargetNamespace string         `xml:"targetNamespace,attr"`
	Types           []*wsdlTypes   `xml:"http://schemas.xmlsoap.org/wsdl/ types"`
	Services        []*wsdlService `xml:"http://schemas.xmlsoap.org/wsdl/ service"`
	Bindings        []*wsdlBinding `xml:"http://schemas.xmlsoap.org/wsdl/ binding"`
}

type wsdlBinding struct {
	Name       string           `xml:"name,attr"`
	Type       string           `xml:"type,attr"`
	Operations []*wsdlOperation `xml:"http://schemas.xmlsoap.org/wsdl/ operation"`
}

type wsdlTypes struct {
	XsdSchema []*xsdSchema `xml:"http://www.w3.org/2001/XMLSchema schema"`
}

type wsdlOperation struct {
	Name           string           `xml:"name,attr"`
	SoapOperations []*soapOperation `xml:"http://schemas.xmlsoap.org/wsdl/soap/ operation"`
}

type wsdlService struct {
	Name  string      `xml:"name,attr"`
	Ports []*wsdlPort `xml:"http://schemas.xmlsoap.org/wsdl/ port"`
}

type wsdlPort struct {
	Name          string         `xml:"name,attr"`
	Binding       string         `xml:"binding,attr"`
	SoapAddresses []*soapAddress `xml:"http://schemas.xmlsoap.org/wsdl/soap/ address"`
}

type soapAddress struct {
	Location string `xml:"location,attr"`
}

type soapOperation struct {
	SoapAction string `xml:"soapAction,attr"`
	Style      string `xml:"style,attr"`
}

type xsdSchema struct {
	TargetNamespace string `xml:"targetNamespace,attr"`
}

func debugSOAP() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("SUBREG_SOAP_DEBUG")))
	return value == "1" || value == "true" || value == "yes"
}

type apiError struct {
	Method  string
	Major   int
	Minor   int
	Message string
}

func (e apiError) Error() string {
	if e.Major == 0 && e.Minor == 0 {
		return fmt.Sprintf("%s failed: %s", e.Method, e.Message)
	}
	return fmt.Sprintf("%s failed (%d/%d): %s", e.Method, e.Major, e.Minor, e.Message)
}

type loginData struct {
	SSID string `xml:"ssid"`
}

type getDNSZoneData struct {
	Domain  string      `xml:"domain"`
	Records []DNSRecord `xml:"records"`
}

type getDNSZoneDataItems struct {
	Domain  string      `xml:"domain"`
	Records []DNSRecord `xml:"records>item"`
}

type getDNSZoneDataRecords struct {
	Domain  string      `xml:"domain"`
	Records []DNSRecord `xml:"records>record"`
}

type getDNSInfoData struct {
	InZone string `xml:"in_zone"`
	DNSSEC string `xml:"dnssec"`
}

type getDomainInfoData struct {
	Domain    string `xml:"domain"`
	Autorenew string `xml:"autorenew"`
}

func New(login, password, wsdlURL string) (*Client, error) {
	if wsdlURL == "" {
		return nil, errors.New("wsdl URL is required")
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     false,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}

	httpClient := &http.Client{Timeout: defaultTimeout, Transport: transport}
	defs, err := getWsdlDefinitions(wsdlURL, httpClient)
	if err != nil {
		return nil, err
	}

	endpointURL, targetNamespace, err := wsdlEndpointAndNamespace(defs)
	if err != nil {
		return nil, err
	}

	return &Client{
		httpClient:      httpClient,
		endpointURL:     endpointURL,
		targetNamespace: targetNamespace,
		wsdl:            defs,
		login:           login,
		password:        password,
	}, nil
}

func (c *Client) Login(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ssid != "" {
		return c.ssid, nil
	}

	resp, err := c.call(ctx, "Login", map[string]any{
		"login":    c.login,
		"password": c.password,
	})
	if err != nil {
		return "", err
	}

	var data loginData
	if err := decodeXMLBytes(resp.Data.InnerXML, &data); err != nil {
		return "", err
	}
	ssid := data.SSID
	if ssid == "" {
		if value, ok := findElementValue(resp.Data.InnerXML, "ssid"); ok {
			ssid = value
		}
	}
	if ssid == "" {
		return "", fmt.Errorf("login response missing ssid (data_len=%d)", len(resp.Data.InnerXML))
	}

	c.ssid = ssid
	return c.ssid, nil
}

func (c *Client) GetDNSZone(ctx context.Context, domain string) ([]DNSRecord, error) {
	ssID, err := c.Login(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := c.call(ctx, "Get_DNS_Zone", map[string]any{
		"ssid":   ssID,
		"domain": domain,
	})
	if err != nil {
		return nil, err
	}

	if debugSOAP() {
		_ = writeDebugFile("subreg_get_dns_zone_data.xml", resp.Data.InnerXML)
	}

	return decodeDNSZoneRecords(resp.Data.InnerXML)
}

func (c *Client) GetDNSInfo(ctx context.Context, domain string) (DNSInfo, error) {
	ssID, err := c.Login(ctx)
	if err != nil {
		return DNSInfo{}, err
	}

	resp, err := c.call(ctx, "Get_DNS_Info", map[string]any{
		"ssid":   ssID,
		"domain": domain,
	})
	if err != nil {
		return DNSInfo{}, err
	}

	return decodeDNSInfo(resp.Data.InnerXML)
}

func (c *Client) SignDNSZone(ctx context.Context, domain string) error {
	ssID, err := c.Login(ctx)
	if err != nil {
		return err
	}

	_, err = c.call(ctx, "Sign_DNS_Zone", map[string]any{
		"ssid":   ssID,
		"domain": domain,
	})
	return err
}

func (c *Client) UnsignDNSZone(ctx context.Context, domain string) error {
	ssID, err := c.Login(ctx)
	if err != nil {
		return err
	}

	_, err = c.call(ctx, "Unsign_DNS_Zone", map[string]any{
		"ssid":   ssID,
		"domain": domain,
	})
	return err
}

func (c *Client) GetDomainInfo(ctx context.Context, domain string) (DomainInfo, error) {
	ssID, err := c.Login(ctx)
	if err != nil {
		return DomainInfo{}, err
	}

	resp, err := c.call(ctx, "Info_Domain", map[string]any{
		"ssid":   ssID,
		"domain": domain,
	})
	if err != nil {
		return DomainInfo{}, err
	}

	return decodeDomainInfo(resp.Data.InnerXML)
}

func (c *Client) SetAutorenew(ctx context.Context, domain string, enabled bool) error {
	ssID, err := c.Login(ctx)
	if err != nil {
		return err
	}

	_, err = c.call(ctx, "Set_Autorenew", map[string]any{
		"ssid":      ssID,
		"domain":    domain,
		"autorenew": subregAutorenewValue(enabled),
	})
	return err
}

func writeDebugFile(name string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	path := fmt.Sprintf("%s/%s", strings.TrimRight(os.TempDir(), "/"), name)
	return os.WriteFile(path, data, 0o600)
}

func decodeDNSZoneRecords(data []byte) ([]DNSRecord, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if records, ok := decodeDNSZonePayload(trimmed); ok {
		return records, nil
	}
	if records, ok := decodeDNSZonePayload(wrapXMLFragment(trimmed)); ok {
		return records, nil
	}

	fallback := parseDNSRecordsFromXML(trimmed)
	if len(fallback) > 0 {
		return fallback, nil
	}

	return nil, fmt.Errorf("unable to parse DNS zone response")
}

func decodeDNSZonePayload(data []byte) ([]DNSRecord, bool) {
	var parsed getDNSZoneData
	if err := decodeXMLBytes(data, &parsed); err == nil {
		records := cleanDNSRecords(parsed.Records)
		if parsed.Domain != "" || len(records) > 0 {
			return records, true
		}
	}

	var parsedItems getDNSZoneDataItems
	if err := decodeXMLBytes(data, &parsedItems); err == nil {
		records := cleanDNSRecords(parsedItems.Records)
		if parsedItems.Domain != "" || len(records) > 0 {
			return records, true
		}
	}

	var parsedRecords getDNSZoneDataRecords
	if err := decodeXMLBytes(data, &parsedRecords); err == nil {
		records := cleanDNSRecords(parsedRecords.Records)
		if parsedRecords.Domain != "" || len(records) > 0 {
			return records, true
		}
	}

	return nil, false
}

func decodeDNSInfo(data []byte) (DNSInfo, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return DNSInfo{}, fmt.Errorf("unable to parse DNS info response")
	}

	var parsed getDNSInfoData
	if err := decodeXMLBytes(wrapXMLFragment(trimmed), &parsed); err != nil {
		return DNSInfo{}, fmt.Errorf("unable to parse DNS info response: %w", err)
	}

	inZone, err := parseSubregBool(parsed.InZone)
	if err != nil {
		return DNSInfo{}, fmt.Errorf("unable to parse in_zone value %q: %w", parsed.InZone, err)
	}

	return DNSInfo{InZone: inZone, DNSSEC: inZone}, nil
}

func decodeDomainInfo(data []byte) (DomainInfo, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return DomainInfo{}, fmt.Errorf("unable to parse domain info response")
	}

	var parsed getDomainInfoData
	if err := decodeXMLBytes(wrapXMLFragment(trimmed), &parsed); err != nil {
		return DomainInfo{}, fmt.Errorf("unable to parse domain info response: %w", err)
	}

	autorenew, err := parseSubregBool(parsed.Autorenew)
	if err != nil {
		return DomainInfo{}, fmt.Errorf("unable to parse autorenew value %q: %w", parsed.Autorenew, err)
	}

	return DomainInfo{Domain: parsed.Domain, Autorenew: autorenew}, nil
}

func cleanDNSRecords(records []DNSRecord) []DNSRecord {
	filtered := make([]DNSRecord, 0, len(records))
	for _, record := range records {
		if record.ID == 0 && record.Name == "" && record.Type == "" && record.Content == "" && record.Prio == 0 && record.TTL == 0 {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

func parseSubregBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "signed", "autorenew", "renewonce":
		return true, nil
	case "0", "false", "no", "off", "unsigned", "expire", "":
		return false, nil
	default:
		return false, fmt.Errorf("unsupported boolean value")
	}
}

func subregAutorenewValue(value bool) string {
	if value {
		return "AUTORENEW"
	}
	return "EXPIRE"
}

func wrapXMLFragment(data []byte) []byte {
	wrapped := make([]byte, 0, len(data)+13)
	wrapped = append(wrapped, []byte("<root>")...)
	wrapped = append(wrapped, data...)
	wrapped = append(wrapped, []byte("</root>")...)
	return wrapped
}

func parseDNSRecordsFromXML(data []byte) []DNSRecord {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	var records []DNSRecord
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return records
			}
			return records
		}

		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "records" {
			continue
		}

		var record DNSRecord
		if err := dec.DecodeElement(&record, &start); err != nil {
			continue
		}
		if record.ID == 0 && record.Type == "" && record.Content == "" && record.Name == "" {
			continue
		}
		records = append(records, record)
	}
}

func (c *Client) AddDNSRecord(ctx context.Context, domain string, record DNSRecordInput) error {
	ssID, err := c.Login(ctx)
	if err != nil {
		return err
	}

	recordParams := map[string]any{
		"name":    record.Name,
		"type":    record.Type,
		"content": record.Content,
	}
	if record.Prio != nil {
		recordParams["prio"] = strconv.Itoa(*record.Prio)
	}
	if record.TTL != nil {
		recordParams["ttl"] = strconv.Itoa(*record.TTL)
	}

	_, err = c.call(ctx, "Add_DNS_Record", map[string]any{
		"ssid":   ssID,
		"domain": domain,
		"record": recordParams,
	})

	return err
}

func (c *Client) AddDNSRecordWithID(ctx context.Context, domain string, record DNSRecordInput) (int, error) {
	if err := c.AddDNSRecord(ctx, domain, record); err != nil {
		return 0, err
	}

	records, err := c.GetDNSZone(ctx, domain)
	if err != nil {
		return 0, err
	}

	return matchCreatedDNSRecordID(records, record)
}

func matchCreatedDNSRecordID(records []DNSRecord, record DNSRecordInput) (int, error) {
	var candidates []DNSRecord
	for _, rec := range records {
		if normalizeRecordNameForMatch(rec.Name) != normalizeRecordNameForMatch(record.Name) {
			continue
		}
		if !strings.EqualFold(rec.Type, record.Type) {
			continue
		}
		if record.Content != "" && rec.Content != record.Content {
			continue
		}
		if record.Prio != nil && rec.Prio != *record.Prio {
			continue
		}
		if record.TTL != nil && rec.TTL != *record.TTL {
			continue
		}
		candidates = append(candidates, rec)
	}

	switch len(candidates) {
	case 0:
		return 0, errors.New("record not found after create")
	case 1:
		return candidates[0].ID, nil
	default:
		return 0, fmt.Errorf("multiple DNS records matched created record (name=%q type=%q content=%q)", record.Name, record.Type, record.Content)
	}
}

func normalizeRecordNameForMatch(value string) string {
	name := strings.TrimSpace(value)
	if name == "" || name == "@" {
		return "@"
	}
	return name
}

func (c *Client) ModifyDNSRecord(ctx context.Context, domain string, id int, record DNSRecordInput) error {
	ssID, err := c.Login(ctx)
	if err != nil {
		return err
	}

	recordParams := map[string]any{
		"id":      strconv.Itoa(id),
		"type":    record.Type,
		"content": record.Content,
	}
	if record.Prio != nil {
		recordParams["prio"] = strconv.Itoa(*record.Prio)
	}
	if record.TTL != nil {
		recordParams["ttl"] = strconv.Itoa(*record.TTL)
	}

	_, err = c.call(ctx, "Modify_DNS_Record", map[string]any{
		"ssid":   ssID,
		"domain": domain,
		"record": recordParams,
	})

	return err
}

func (c *Client) DeleteDNSRecord(ctx context.Context, domain string, id int) error {
	ssID, err := c.Login(ctx)
	if err != nil {
		return err
	}

	_, err = c.call(ctx, "Delete_DNS_Record", map[string]any{
		"ssid":   ssID,
		"domain": domain,
		"record": map[string]any{"id": strconv.Itoa(id)},
	})

	return err
}

func (c *Client) GetDNSRecordByID(ctx context.Context, domain string, id int) (DNSRecord, bool, error) {
	records, err := c.GetDNSZone(ctx, domain)
	if err != nil {
		return DNSRecord{}, false, err
	}

	for _, record := range records {
		if record.ID == id {
			return record, true, nil
		}
	}

	return DNSRecord{}, false, nil
}

func (c *Client) call(ctx context.Context, method string, params any) (responseBody, error) {
	if c.httpClient == nil {
		return responseBody{}, errors.New("http client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return responseBody{}, err
	}
	if c.endpointURL == "" {
		return responseBody{}, errors.New("soap endpoint is not initialized")
	}

	soapAction := ""
	if c.wsdl != nil {
		soapAction = c.wsdl.GetSoapActionFromWsdlOperation(method)
	}
	if soapAction == "" {
		return responseBody{}, fmt.Errorf("soap action not found for %s", method)
	}

	payload, err := buildSOAPEnvelope(method, c.targetNamespace, params)
	if err != nil {
		return responseBody{}, fmt.Errorf("%s payload build failed: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL, bytes.NewReader(payload))
	if err != nil {
		return responseBody{}, fmt.Errorf("%s request build failed: %w", method, err)
	}
	req.Header.Set("Content-Type", "text/xml;charset=UTF-8")
	req.Header.Set("Accept", "text/xml")
	req.Header.Set("SOAPAction", soapAction)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return responseBody{}, fmt.Errorf("%s call failed: %w", method, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return responseBody{}, fmt.Errorf("%s response read failed: %w", method, err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 400 {
		return responseBody{}, fmt.Errorf("%s call failed: unexpected status code: %s", method, res.Status)
	}

	var soapResp soapEnvelope
	if err := decodeXMLBytes(body, &soapResp); err != nil {
		return responseBody{}, fmt.Errorf("%s soap envelope unmarshal failed: %w", method, err)
	}

	var envelope responseEnvelope
	if err := decodeXMLBytes(soapResp.Body.InnerXML, &envelope); err != nil {
		return responseBody{}, fmt.Errorf("%s response unmarshal failed: %w", method, err)
	}

	if envelope.Response.Status == "" {
		return responseBody{}, fmt.Errorf("empty response status for %s", method)
	}
	if envelope.Response.Status != "ok" {
		return responseBody{}, apiErrorFromResponse(method, envelope.Response)
	}

	return envelope.Response, nil
}

func buildSOAPEnvelope(method, namespace string, params any) ([]byte, error) {
	if method == "" || namespace == "" {
		return nil, errors.New("method or namespace is empty")
	}

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	tokens := tokenData{}
	tokens.startEnvelope()
	if err := tokens.startBody(method, namespace); err != nil {
		return nil, err
	}
	tokens.recursiveEncode(params)
	tokens.endBody(method)
	tokens.endEnvelope()

	for _, token := range tokens.data {
		if err := enc.EncodeToken(token); err != nil {
			return nil, err
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type tokenData struct {
	data []xml.Token
}

func (tokens *tokenData) recursiveEncode(value any) {
	if value == nil {
		return
	}

	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		for _, key := range v.MapKeys() {
			t := xml.StartElement{Name: xml.Name{Local: fmt.Sprint(key.Interface())}}
			tokens.data = append(tokens.data, t)
			tokens.recursiveEncode(v.MapIndex(key).Interface())
			tokens.data = append(tokens.data, xml.EndElement{Name: t.Name})
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			tokens.recursiveEncode(v.Index(i).Interface())
		}
	case reflect.String:
		tokens.data = append(tokens.data, xml.CharData(v.String()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		tokens.data = append(tokens.data, xml.CharData(strconv.FormatInt(v.Int(), 10)))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		tokens.data = append(tokens.data, xml.CharData(strconv.FormatUint(v.Uint(), 10)))
	case reflect.Bool:
		tokens.data = append(tokens.data, xml.CharData(strconv.FormatBool(v.Bool())))
	default:
		tokens.data = append(tokens.data, xml.CharData(fmt.Sprint(v.Interface())))
	}
}

func (tokens *tokenData) startEnvelope() {
	e := xml.StartElement{
		Name: xml.Name{Local: soapPrefix + ":Envelope"},
	}
	e.Attr = []xml.Attr{
		{Name: xml.Name{Local: "xmlns:xsi"}, Value: "http://www.w3.org/2001/XMLSchema-instance"},
		{Name: xml.Name{Local: "xmlns:xsd"}, Value: "http://www.w3.org/2001/XMLSchema"},
		{Name: xml.Name{Local: "xmlns:soap"}, Value: "http://schemas.xmlsoap.org/soap/envelope/"},
	}
	tokens.data = append(tokens.data, e)
}

func (tokens *tokenData) endEnvelope() {
	tokens.data = append(tokens.data, xml.EndElement{Name: xml.Name{Local: soapPrefix + ":Envelope"}})
}

func (tokens *tokenData) startBody(method, namespace string) error {
	b := xml.StartElement{Name: xml.Name{Local: soapPrefix + ":Body"}}
	r := xml.StartElement{
		Name: xml.Name{Local: method},
		Attr: []xml.Attr{{Name: xml.Name{Local: "xmlns"}, Value: namespace}},
	}
	tokens.data = append(tokens.data, b, r)
	return nil
}

func (tokens *tokenData) endBody(method string) {
	tokens.data = append(tokens.data,
		xml.EndElement{Name: xml.Name{Local: method}},
		xml.EndElement{Name: xml.Name{Local: soapPrefix + ":Body"}},
	)
}

func decodeXMLBytes(data []byte, target any) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	return dec.Decode(target)
}

func getWsdlDefinitions(u string, c *http.Client) (*wsdlDefinitions, error) {
	reader, err := getWsdlBody(u, c)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = charset.NewReaderLabel
	var wsdl wsdlDefinitions
	if err := decoder.Decode(&wsdl); err != nil {
		return nil, err
	}

	return &wsdl, nil
}

func getWsdlBody(u string, c *http.Client) (reader io.ReadCloser, err error) {
	parse, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if parse.Scheme == "file" {
		outFile, err := os.Open(parse.Path)
		if err != nil {
			return nil, err
		}
		return outFile, nil
	}
	if c == nil {
		c = &http.Client{}
	}
	r, err := c.Get(u)
	if err != nil {
		return nil, err
	}
	return r.Body, nil
}

func wsdlEndpointAndNamespace(defs *wsdlDefinitions) (string, string, error) {
	if defs == nil {
		return "", "", errors.New("wsdl definitions not found")
	}
	if len(defs.Types) == 0 || defs.Types[0] == nil || len(defs.Types[0].XsdSchema) == 0 || defs.Types[0].XsdSchema[0] == nil {
		return "", "", errors.New("wsdl target namespace not found")
	}
	namespace := defs.Types[0].XsdSchema[0].TargetNamespace
	if namespace == "" {
		return "", "", errors.New("wsdl target namespace not found")
	}
	if len(defs.Services) == 0 || defs.Services[0] == nil {
		return "", "", errors.New("wsdl service not found")
	}
	if len(defs.Services[0].Ports) == 0 || defs.Services[0].Ports[0] == nil {
		return "", "", errors.New("wsdl port not found")
	}
	if len(defs.Services[0].Ports[0].SoapAddresses) == 0 || defs.Services[0].Ports[0].SoapAddresses[0] == nil {
		return "", "", errors.New("wsdl soap address not found")
	}
	endpointURL := defs.Services[0].Ports[0].SoapAddresses[0].Location
	if endpointURL == "" {
		return "", "", errors.New("wsdl soap endpoint not found")
	}
	return endpointURL, namespace, nil
}

func (wsdl *wsdlDefinitions) GetSoapActionFromWsdlOperation(operation string) string {
	if wsdl == nil || len(wsdl.Bindings) == 0 || wsdl.Bindings[0] == nil {
		return ""
	}
	for _, o := range wsdl.Bindings[0].Operations {
		if o == nil || o.Name != operation {
			continue
		}
		if len(o.SoapOperations) > 0 && o.SoapOperations[0] != nil {
			return o.SoapOperations[0].SoapAction
		}
	}
	return ""
}

func findElementValue(data []byte, name string) (string, bool) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	for {
		ok, value, err := decodeNextElementValue(dec, name)
		if err != nil {
			return "", false
		}
		if ok {
			return value, true
		}
	}
}

func decodeNextElementValue(dec *xml.Decoder, name string) (bool, string, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return false, "", io.EOF
			}
			return false, "", err
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		if start.Name.Local != name {
			continue
		}

		var value string
		if err := dec.DecodeElement(&value, &start); err != nil {
			return false, "", err
		}
		return true, value, nil
	}
}

func apiErrorFromResponse(method string, resp responseBody) error {
	if resp.Error == nil {
		return apiError{Method: method, Message: "unknown error"}
	}

	errCode := resp.Error.ErrorCode
	if errCode == nil {
		return apiError{Method: method, Message: resp.Error.ErrorMsg}
	}

	return apiError{
		Method:  method,
		Major:   errCode.Major,
		Minor:   errCode.Minor,
		Message: resp.Error.ErrorMsg,
	}
}
