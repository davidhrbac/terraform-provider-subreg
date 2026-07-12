package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/davidhrbac/terraform-provider-subreg/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dnsRecordResource struct {
	client *client.Client
}

type dnsRecordResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Domain  types.String `tfsdk:"domain"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Content types.String `tfsdk:"content"`
	Prio    types.Int64  `tfsdk:"prio"`
	TTL     types.Int64  `tfsdk:"ttl"`
}

func NewDNSRecordResource() resource.Resource {
	return &dnsRecordResource{}
}

func (r *dnsRecordResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *dnsRecordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"domain": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description: "Registered domain that owns the record.",
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description: "Record name without the registered domain (e.g. @, www).",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "DNS record type (A, AAAA, CNAME, MX, TXT, etc.).",
			},
			"content": schema.StringAttribute{
				Required:    true,
				Description: "Record value (IP, hostname, or text value).",
			},
			"prio": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Priority for MX records.",
			},
			"ttl": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "TTL for the record in seconds.",
			},
		},
	}
}

func (r *dnsRecordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientData, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "Expected *client.Client")
		return
	}

	r.client = clientData
}

func (r *dnsRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateTTL(plan.TTL)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordInput := client.DNSRecordInput{
		Name:    apiRecordName(plan.Name.ValueString()),
		Type:    normalizeRecordType(plan.Type.ValueString()),
		Content: plan.Content.ValueString(),
		Prio:    optionalInt(plan.Prio),
		TTL:     optionalInt(plan.TTL),
	}

	id, err := r.client.AddDNSRecordWithID(ctx, plan.Domain.ValueString(), recordInput)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create DNS record", err.Error())
		return
	}

	record, found, err := r.client.GetDNSRecordByID(ctx, plan.Domain.ValueString(), id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read created DNS record", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Unable to read created DNS record", "record not found after create")
		return
	}

	state := dnsRecordResourceModel{
		ID:      types.StringValue(strconv.Itoa(id)),
		Domain:  plan.Domain,
		Name:    types.StringValue(normalizeRecordName(record.Name)),
		Type:    types.StringValue(normalizeRecordType(record.Type)),
		Content: types.StringValue(record.Content),
		Prio:    recordPriorityValue(record.Prio),
		TTL:     types.Int64Value(int64(record.TTL)),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordID, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid DNS record ID", err.Error())
		return
	}

	record, found, err := r.client.GetDNSRecordByID(ctx, state.Domain.ValueString(), recordID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read DNS record", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(normalizeRecordName(record.Name))
	state.Type = types.StringValue(normalizeRecordType(record.Type))
	state.Content = types.StringValue(record.Content)
	state.Prio = recordPriorityValue(record.Prio)
	state.TTL = types.Int64Value(int64(record.TTL))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnsRecordResourceModel
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateTTL(plan.TTL)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordID, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid DNS record ID", err.Error())
		return
	}

	recordInput := client.DNSRecordInput{
		Name:    apiRecordName(plan.Name.ValueString()),
		Type:    normalizeRecordType(plan.Type.ValueString()),
		Content: plan.Content.ValueString(),
		Prio:    optionalInt(plan.Prio),
		TTL:     optionalInt(plan.TTL),
	}

	err = r.client.ModifyDNSRecord(ctx, plan.Domain.ValueString(), recordID, recordInput)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update DNS record", err.Error())
		return
	}

	record, found, err := r.client.GetDNSRecordByID(ctx, plan.Domain.ValueString(), recordID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read updated DNS record", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Updated DNS record not found", fmt.Sprintf("Record id %d not found", recordID))
		return
	}

	state.Name = types.StringValue(normalizeRecordName(record.Name))
	state.Type = types.StringValue(normalizeRecordType(record.Type))
	state.Content = types.StringValue(record.Content)
	state.Prio = recordPriorityValue(record.Prio)
	state.TTL = types.Int64Value(int64(record.TTL))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordID, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid DNS record ID", err.Error())
		return
	}

	err = r.client.DeleteDNSRecord(ctx, state.Domain.ValueString(), recordID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete DNS record", err.Error())
		return
	}
}

func (r *dnsRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Unexpected import format", "Use domain:id (example.com:123)")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func normalizeRecordType(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeRecordName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return "@"
	}
	if name == "@" {
		return "@"
	}
	return name
}

func apiRecordName(value string) string {
	name := strings.TrimSpace(value)
	if name == "@" {
		return ""
	}
	return name
}

func validateTTL(value types.Int64) diag.Diagnostics {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return diags
	}
	if value.ValueInt64() == 0 {
		return diags
	}
	if value.ValueInt64() < 600 {
		diags.AddError(
			"Invalid TTL",
			"Subreg enforces a minimum TTL of 600 seconds (or 0 for default).",
		)
	}
	return diags
}

func optionalInt(value types.Int64) *int {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	if value.ValueInt64() == 0 {
		return nil
	}
	v := int(value.ValueInt64())
	return &v
}

func recordPriorityValue(value int) types.Int64 {
	if value == 0 {
		return types.Int64Null()
	}
	return types.Int64Value(int64(value))
}
