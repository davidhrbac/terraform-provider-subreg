package main

import (
	"context"
	"flag"

	"github.com/davidhrbac/terraform-provider-subreg/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "enable provider debug")
	flag.Parse()

	providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/davidhrbac/subreg",
		Debug:   debug,
	})
}
