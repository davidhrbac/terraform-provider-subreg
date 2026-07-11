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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tiaguinho/gosoap"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	soap     *gosoap.Client
	login    string
	password string

	mu   sync.Mutex
	ssid string
}

type responseEnvelope struct {
	Response responseBody `xml:"response"`
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
	soap, err := gosoap.SoapClient(wsdlURL, httpClient)
	if err != nil {
		return nil, err
	}

	return &Client{
		soap:     soap,
		login:    login,
		password: password,
	}, nil
}

func (c *Client) Login(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ssid != "" {
		return c.ssid, nil
	}

	resp, err := c.call(ctx, "Login", gosoap.Params{
		"login":    c.login,
		"password": c.password,
	})
	if err != nil {
		return "", err
	}

	var data loginData
	if err := xml.Unmarshal(resp.Data.InnerXML, &data); err != nil {
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
	ssid, err := c.Login(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := c.call(ctx, "Get_DNS_Zone", gosoap.Params{
		"ssid":   ssid,
		"domain": domain,
	})
	if err != nil {
		return nil, err
	}

	if debugSOAP() {
		_ = writeDebugFile("subreg_get_dns_zone_data.xml", resp.Data.InnerXML)
	}

	var data getDNSZoneData
	if err := xml.Unmarshal(resp.Data.InnerXML, &data); err != nil {
		return nil, err
	}

	if len(data.Records) > 0 {
		return data.Records, nil
	}

	var dataItems getDNSZoneDataItems
	if err := xml.Unmarshal(resp.Data.InnerXML, &dataItems); err == nil && len(dataItems.Records) > 0 {
		return dataItems.Records, nil
	}

	var dataRecords getDNSZoneDataRecords
	if err := xml.Unmarshal(resp.Data.InnerXML, &dataRecords); err == nil && len(dataRecords.Records) > 0 {
		return dataRecords.Records, nil
	}

	fallback := parseDNSRecordsFromXML(resp.Data.InnerXML)
	if len(fallback) > 0 {
		return fallback, nil
	}

	return nil, nil
}

func writeDebugFile(name string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	path := fmt.Sprintf("%s/%s", strings.TrimRight(os.TempDir(), "/"), name)
	return os.WriteFile(path, data, 0o600)
}

func parseDNSRecordsFromXML(data []byte) []DNSRecord {
	dec := xml.NewDecoder(bytes.NewReader(data))
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
	ssid, err := c.Login(ctx)
	if err != nil {
		return err
	}

	recordParams := gosoap.Params{
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

	_, err = c.call(ctx, "Add_DNS_Record", gosoap.Params{
		"ssid":   ssid,
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

	var candidates []DNSRecord
	for _, rec := range records {
		if rec.Name != record.Name {
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

	if len(candidates) == 0 {
		return 0, errors.New("record not found after create")
	}

	// Prefer the newest record when duplicates exist.
	maxID := candidates[0].ID
	for _, rec := range candidates[1:] {
		if rec.ID > maxID {
			maxID = rec.ID
		}
	}

	return maxID, nil
}

func (c *Client) ModifyDNSRecord(ctx context.Context, domain string, id int, record DNSRecordInput) error {
	ssid, err := c.Login(ctx)
	if err != nil {
		return err
	}

	recordParams := gosoap.Params{
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

	_, err = c.call(ctx, "Modify_DNS_Record", gosoap.Params{
		"ssid":   ssid,
		"domain": domain,
		"record": recordParams,
	})

	return err
}

func (c *Client) DeleteDNSRecord(ctx context.Context, domain string, id int) error {
	ssid, err := c.Login(ctx)
	if err != nil {
		return err
	}

	_, err = c.call(ctx, "Delete_DNS_Record", gosoap.Params{
		"ssid":   ssid,
		"domain": domain,
		"record": gosoap.Params{"id": strconv.Itoa(id)},
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

func (c *Client) call(ctx context.Context, method string, params gosoap.Params) (responseBody, error) {
	if c.soap == nil {
		return responseBody{}, errors.New("soap client is not initialized")
	}

	res, err := c.soap.Call(method, params)
	if err != nil {
		return responseBody{}, fmt.Errorf("%s call failed: %w", method, err)
	}

	var envelope responseEnvelope
	if err := res.Unmarshal(&envelope); err != nil {
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

func findElementValue(data []byte, name string) (string, bool) {
	dec := xml.NewDecoder(bytes.NewReader(data))
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
