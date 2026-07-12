package provider

import (
	"context"
	"fmt"

	"github.com/davidhrbac/terraform-provider-subreg/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dnsZoneResource struct {
	client *client.Client
}

type dnsZoneResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Domain types.String `tfsdk:"domain"`
	DNSSEC types.Bool   `tfsdk:"dnssec"`
}

func NewDNSZoneResource() resource.Resource {
	return &dnsZoneResource{}
}

func (r *dnsZoneResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone"
}

func (r *dnsZoneResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				Description: "Registered domain whose DNSSEC signing state is managed.",
			},
			"dnssec": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the zone is DNSSEC signed.",
			},
		},
	}
}

func (r *dnsZoneResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dnsZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.ensureDNSSECEnabled(ctx, plan.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to enable DNSSEC for DNS zone", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	refreshed, keep, err := r.readDNSSECState(ctx, state.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read DNS zone DNSSEC state", err.Error())
		return
	}
	if !keep {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *dnsZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state dnsZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	refreshed, keep, err := r.readDNSSECState(ctx, state.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to refresh DNS zone DNSSEC state", err.Error())
		return
	}
	if !keep {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *dnsZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	info, err := r.client.GetDNSInfo(ctx, state.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read DNS zone DNSSEC state", err.Error())
		return
	}
	if !info.DNSSEC {
		return
	}

	if err := r.client.UnsignDNSZone(ctx, state.Domain.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to disable DNSSEC for DNS zone", err.Error())
		return
	}
}

func (r *dnsZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *dnsZoneResource) ensureDNSSECEnabled(ctx context.Context, domain string) (dnsZoneResourceModel, error) {
	info, err := r.client.GetDNSInfo(ctx, domain)
	if err != nil {
		return dnsZoneResourceModel{}, err
	}
	if !info.InZone {
		return dnsZoneResourceModel{}, fmt.Errorf("domain %q is not present in the DNS zone", domain)
	}
	if !info.DNSSEC {
		if err := r.client.SignDNSZone(ctx, domain); err != nil {
			return dnsZoneResourceModel{}, err
		}
		info, err = r.client.GetDNSInfo(ctx, domain)
		if err != nil {
			return dnsZoneResourceModel{}, err
		}
		if !info.DNSSEC {
			return dnsZoneResourceModel{}, fmt.Errorf("DNSSEC is still disabled for %q after signing", domain)
		}
	}

	return dnsZoneResourceModel{
		ID:     types.StringValue(domain),
		Domain: types.StringValue(domain),
		DNSSEC: types.BoolValue(true),
	}, nil
}

func (r *dnsZoneResource) readDNSSECState(ctx context.Context, domain string) (dnsZoneResourceModel, bool, error) {
	info, err := r.client.GetDNSInfo(ctx, domain)
	if err != nil {
		return dnsZoneResourceModel{}, false, err
	}
	if !info.InZone || !info.DNSSEC {
		return dnsZoneResourceModel{}, false, nil
	}

	return dnsZoneResourceModel{
		ID:     types.StringValue(domain),
		Domain: types.StringValue(domain),
		DNSSEC: types.BoolValue(true),
	}, true, nil
}
