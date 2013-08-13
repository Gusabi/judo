// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package imagemetadata

import (
	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/environs/config"
	coretesting "launchpad.net/juju-core/testing"
	"net/http"
)

type ValidateSuite struct {
	home      *coretesting.FakeHome
	oldClient *http.Client
}

var _ = gc.Suite(&ValidateSuite{})

func (s *ValidateSuite) makeLocalMetadata(c *gc.C, id, region, series, endpoint string) error {
	im := ImageMetadata{
		Id:   id,
		Arch: "amd64",
	}
	cloudSpec := CloudSpec{
		Region:   region,
		Endpoint: endpoint,
	}
	_, err := MakeBoilerplate("", series, &im, &cloudSpec, false)
	if err != nil {
		return err
	}

	t := &http.Transport{}
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
	s.oldClient = SetHttpClient(&http.Client{Transport: t})
	return nil
}

func (s *ValidateSuite) SetUpTest(c *gc.C) {
	s.home = coretesting.MakeEmptyFakeHome(c)
}

func (s *ValidateSuite) TearDownTest(c *gc.C) {
	s.home.Restore()
	if s.oldClient != nil {
		SetHttpClient(s.oldClient)
	}
}

func (s *ValidateSuite) TestMatch(c *gc.C) {
	s.makeLocalMetadata(c, "1234", "region-2", "raring", "some-auth-url")
	metadataDir := config.JujuHomePath("")
	params := &MetadataLookupParams{
		Region:        "region-2",
		Series:        "raring",
		Architectures: []string{"amd64"},
		Endpoint:      "some-auth-url",
		BaseURLs:      []string{"file://" + metadataDir},
	}
	imageIds, err := ValidateImageMetadata(params)
	c.Assert(err, gc.IsNil)
	c.Assert(imageIds, gc.DeepEquals, []string{"1234"})
}

func (s *ValidateSuite) TestNoMatch(c *gc.C) {
	s.makeLocalMetadata(c, "1234", "region-2", "raring", "some-auth-url")
	metadataDir := config.JujuHomePath("")
	params := &MetadataLookupParams{
		Region:        "region-2",
		Series:        "precise",
		Architectures: []string{"amd64"},
		Endpoint:      "some-auth-url",
		BaseURLs:      []string{"file://" + metadataDir},
	}
	_, err := ValidateImageMetadata(params)
	c.Assert(err, gc.Not(gc.IsNil))
}
