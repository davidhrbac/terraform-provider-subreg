package client

type DNSRecord struct {
	ID      int    `xml:"id"`
	Name    string `xml:"name"`
	Type    string `xml:"type"`
	Content string `xml:"content"`
	Prio    int    `xml:"prio"`
	TTL     int    `xml:"ttl"`
}

type DNSRecordInput struct {
	Name    string
	Type    string
	Content string
	Prio    *int
	TTL     *int
}
