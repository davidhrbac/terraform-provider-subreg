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

type domainResource struct {
	client *client.Client
}

type domainResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Domain    types.String `tfsdk:"domain"`
	Autorenew types.Bool   `tfsdk:"autorenew"`
}

func NewDomainResource() resource.Resource {
	return &domainResource{}
}

func (r *domainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *domainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				Description: "Registered domain to manage.",
			},
			"autorenew": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the domain should auto-renew.",
			},
		},
	}
}

func (r *domainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.applyAutorenew(ctx, plan.Domain.ValueString(), plan.Autorenew.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure domain autorenew", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	refreshed, keep, err := r.refreshDomain(ctx, state.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read domain state", err.Error())
		return
	}
	if !keep {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.applyAutorenew(ctx, plan.Domain.ValueString(), plan.Autorenew.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("Unable to update domain autorenew", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetAutorenew(ctx, state.Domain.ValueString(), false); err != nil {
		resp.Diagnostics.AddError("Unable to disable domain autorenew", err.Error())
		return
	}
}

func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *domainResource) applyAutorenew(ctx context.Context, domain string, enabled bool) (domainResourceModel, error) {
	info, err := r.client.GetDomainInfo(ctx, domain)
	if err != nil {
		return domainResourceModel{}, err
	}
	if info.Domain != "" && info.Domain != domain {
		return domainResourceModel{}, fmt.Errorf("unexpected domain info response for %q", domain)
	}

	if info.Autorenew != enabled {
		if err := r.client.SetAutorenew(ctx, domain, enabled); err != nil {
			return domainResourceModel{}, err
		}
		info.Autorenew = enabled
	}

	return domainResourceModel{
		ID:        types.StringValue(domain),
		Domain:    types.StringValue(domain),
		Autorenew: types.BoolValue(info.Autorenew),
	}, nil
}

func (r *domainResource) refreshDomain(ctx context.Context, domain string) (domainResourceModel, bool, error) {
	info, err := r.client.GetDomainInfo(ctx, domain)
	if err != nil {
		return domainResourceModel{}, false, err
	}
	if info.Domain != "" && info.Domain != domain {
		return domainResourceModel{}, false, fmt.Errorf("unexpected domain info response for %q", domain)
	}

	return domainResourceModel{
		ID:        types.StringValue(domain),
		Domain:    types.StringValue(domain),
		Autorenew: types.BoolValue(info.Autorenew),
	}, true, nil
}
