package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/davidhrbac/terraform-provider-subreg/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultWSDLURL = "https://subreg.cz/wsdl"

type subregProvider struct {
	version string
}

type providerModel struct {
	Login    types.String `tfsdk:"login"`
	Password types.String `tfsdk:"password"`
	WSDLURL  types.String `tfsdk:"wsdl_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &subregProvider{version: version}
	}
}

func (p *subregProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "subreg"
	resp.Version = p.version
}

func (p *subregProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"login": schema.StringAttribute{
				Optional:    true,
				Description: "Subreg API login. Can also be set via SUBREG_LOGIN.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Subreg API password. Can also be set via SUBREG_PASSWORD.",
			},
			"wsdl_url": schema.StringAttribute{
				Optional:    true,
				Description: "Subreg WSDL URL. Defaults to production WSDL.",
			},
		},
	}
}

func (p *subregProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	login, diags := configString(config.Login, "SUBREG_LOGIN")
	resp.Diagnostics.Append(diags...)
	password, diags := configString(config.Password, "SUBREG_PASSWORD")
	resp.Diagnostics.Append(diags...)

	wsdlURL := defaultWSDLURL
	if !config.WSDLURL.IsNull() && !config.WSDLURL.IsUnknown() {
		wsdlURL = config.WSDLURL.ValueString()
	} else if envValue := os.Getenv("SUBREG_WSDL_URL"); envValue != "" {
		wsdlURL = envValue
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if login == "" {
		resp.Diagnostics.AddError("Missing Subreg login", "Set provider login or SUBREG_LOGIN.")
		return
	}
	if password == "" {
		resp.Diagnostics.AddError("Missing Subreg password", "Set provider password or SUBREG_PASSWORD.")
		return
	}

	apiClient, err := client.New(login, password, wsdlURL)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Subreg client", err.Error())
		return
	}

	resp.ResourceData = apiClient
	resp.DataSourceData = apiClient
}

func (p *subregProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDNSRecordResource,
		NewDNSZoneResource,
	}
}

func (p *subregProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDNSZoneDataSource,
	}
}

func configString(value types.String, envName string) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsUnknown() {
		diags.AddError("Unknown configuration value", fmt.Sprintf("%s is unknown", envName))
		return "", diags
	}
	if !value.IsNull() {
		return value.ValueString(), diags
	}
	return os.Getenv(envName), diags
}
