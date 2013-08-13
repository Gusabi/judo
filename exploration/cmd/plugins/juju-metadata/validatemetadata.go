// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"launchpad.net/gnuflag"

	"launchpad.net/juju-core/cmd"
	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/environs/imagemetadata"
)

// ValidateImageMetadataCommand
type ValidateImageMetadataCommand struct {
	cmd.EnvCommandBase
	providerType string
	metadataDir  string
	series       string
	region       string
	endpoint     string
}

var validateImagesMetadataDoc = `
validate-images loads simplestreams metadata and validates the contents by looking for images
belonging to the specified cloud.

The cloud specificaton comes from the current Juju environment, as specified in the usual way
from either ~/.juju/environments.yaml, the -e option, or JUJU_ENV. Series, Region, and Endpoint
are the key attributes.

The key environment attributes may be overridden using command arguments, so that the validation
may be peformed on arbitary metadata.

Examples:

- validate using the current environment settings but with series raring
 juju metadata validate-images -s raring

- validate using the current environment settings but with series raring and using metadata from local directory
 juju metadata validate-images -s raring -d <some directory>

A key use case is to validate newly generated metadata prior to deployment to production.
In this case, the metadata is placed in a local directory, a cloud provider type is specified (ec2, openstack etc),
and the validation is performed for each supported region and series.

Example bash snippet:

#!/bin/bash

juju metadata validate-images -p ec2 -r us-east-1 -s precise -d <some directory>
RETVAL=$?
[ $RETVAL -eq 0 ] && echo Success
[ $RETVAL -ne 0 ] && echo Failure
`

func (c *ValidateImageMetadataCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "validate-images",
		Purpose: "validate image metadata and ensure image(s) exist for an environment",
		Doc:     validateImagesMetadataDoc,
	}
}

func (c *ValidateImageMetadataCommand) SetFlags(f *gnuflag.FlagSet) {
	c.EnvCommandBase.SetFlags(f)
	f.StringVar(&c.providerType, "p", "", "the provider type eg ec2, openstack")
	f.StringVar(&c.metadataDir, "d", "", "directory where metadata files are found")
	f.StringVar(&c.series, "s", "", "the series for which to validate (overrides env config series)")
	f.StringVar(&c.region, "r", "", "the region for which to validate (overrides env config region)")
	f.StringVar(&c.endpoint, "u", "", "the cloud endpoint URL for which to validate (overrides env config endpoint)")
}

func (c *ValidateImageMetadataCommand) Init(args []string) error {
	if c.providerType != "" {
		if c.series == "" {
			return fmt.Errorf("series required if provider type is specified")
		}
		if c.region == "" {
			return fmt.Errorf("region required if provider type is specified")
		}
		if c.metadataDir == "" {
			return fmt.Errorf("metadata directory required if provider type is specified")
		}
	}
	return c.EnvCommandBase.Init(args)
}

func (c *ValidateImageMetadataCommand) Run(context *cmd.Context) error {
	var params *imagemetadata.MetadataLookupParams

	if c.providerType == "" {
		environ, err := environs.NewFromName(c.EnvName)
		if err != nil {
			return err
		}
		mdLookup, ok := environ.(imagemetadata.ImageMetadataValidator)
		if !ok {
			return fmt.Errorf("%s provider does not support image metadata validation", environ.Config().Type())
		}
		params, err = mdLookup.MetadataLookupParams(c.region)
		if err != nil {
			return err
		}
	} else {
		prov, err := environs.Provider(c.providerType)
		if err != nil {
			return err
		}
		mdLookup, ok := prov.(imagemetadata.ImageMetadataValidator)
		if !ok {
			return fmt.Errorf("%s provider does not support image metadata validation", c.providerType)
		}
		params, err = mdLookup.MetadataLookupParams(c.region)
		if err != nil {
			return err
		}
	}

	if c.series != "" {
		params.Series = c.series
	}
	if c.region != "" {
		params.Region = c.region
	}
	if c.endpoint != "" {
		params.Endpoint = c.endpoint
	}
	// If the metadata files are to be loaded from a directory, we need to register
	// a file http transport.
	if c.metadataDir != "" {
		if _, err := os.Stat(c.metadataDir); err != nil {
			return err
		}

		params.BaseURLs = []string{"file://" + c.metadataDir}
		t := &http.Transport{}
		t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
		c := &http.Client{Transport: t}
		imagemetadata.SetHttpClient(c)
	}

	image_ids, err := imagemetadata.ValidateImageMetadata(params)
	if err != nil {
		return err
	}

	if len(image_ids) > 0 {
		fmt.Fprintf(context.Stdout, "matching image ids for region %q:\n%s\n", params.Region, strings.Join(image_ids, "\n"))
	} else {
		return fmt.Errorf("no matching image ids for region %s using URLs:\n%s", params.Region, strings.Join(params.BaseURLs, "\n"))
	}
	return nil
}
