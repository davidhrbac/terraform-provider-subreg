package provider

import (
	"context"
	"strconv"

	"github.com/davidhrbac/terraform-provider-subreg/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dnsZoneDataSource struct {
	client *client.Client
}

type dnsZoneDataSourceModel struct {
	Domain  types.String         `tfsdk:"domain"`
	Records []dnsZoneRecordModel `tfsdk:"records"`
}

type dnsZoneRecordModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Content types.String `tfsdk:"content"`
	Prio    types.Int64  `tfsdk:"prio"`
	TTL     types.Int64  `tfsdk:"ttl"`
}

func NewDNSZoneDataSource() datasource.DataSource {
	return &dnsZoneDataSource{}
}

func (d *dnsZoneDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone"
}

func (d *dnsZoneDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				Required:    true,
				Description: "Registered domain to read DNS records from.",
			},
			"records": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
						"type": schema.StringAttribute{
							Computed: true,
						},
						"content": schema.StringAttribute{
							Computed: true,
						},
						"prio": schema.Int64Attribute{
							Computed: true,
						},
						"ttl": schema.Int64Attribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (d *dnsZoneDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientData, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "Expected *client.Client")
		return
	}

	d.client = clientData
}

func (d *dnsZoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config dnsZoneDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	records, err := d.client.GetDNSZone(ctx, config.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read DNS zone", err.Error())
		return
	}

	state := dnsZoneDataSourceModel{
		Domain:  config.Domain,
		Records: make([]dnsZoneRecordModel, 0, len(records)),
	}

	for _, record := range records {
		state.Records = append(state.Records, dnsZoneRecordModel{
			ID:      types.StringValue(strconv.Itoa(record.ID)),
			Name:    types.StringValue(normalizeRecordName(record.Name)),
			Type:    types.StringValue(record.Type),
			Content: types.StringValue(record.Content),
			Prio:    types.Int64Value(int64(record.Prio)),
			TTL:     types.Int64Value(int64(record.TTL)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
