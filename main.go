// terraform-provider-fopost is the official Terraform provider for FoPost.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/fopost/terraform-provider-fopost/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// version is stamped by the release build. It travels in the User-Agent of
// every FoPost API call the provider makes.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider in debug mode, for attaching a debugger")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/fopost/fopost",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
