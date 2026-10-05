// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"terraform-provider-jiraassets/internal/provider"
)

// Run "go generate" to format the example configs and regenerate the docs.
//
// tfplugindocs would otherwise shell out to a terraform binary to export the
// schema, and download one if absent. generate.sh instead exports the schema
// with tofu and passes it in via -providers-schema, so no terraform is needed.
//go:generate tofu fmt -recursive ./examples/
//go:generate ./scripts/generate.sh

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary.
	version string = "dev"

	// goreleaser can pass other information to the main package, such as the specific commit
	// https://goreleaser.com/cookbooks/using-main.version/
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/forevanyeung/jiraassets",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)

	if err != nil {
		log.Fatal(err.Error())
	}
}
