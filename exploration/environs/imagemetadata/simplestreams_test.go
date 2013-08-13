// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package imagemetadata

import (
	"bytes"
	"flag"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"launchpad.net/goamz/aws"
	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/environs/jujutest"
	coretesting "launchpad.net/juju-core/testing"
)

var live = flag.Bool("live", false, "Include live simplestreams tests")
var vendor = flag.String("vendor", "", "The vendor representing the source of the simplestream data")

type liveTestData struct {
	baseURL        string
	requireSigned  bool
	validCloudSpec CloudSpec
}

var liveUrls = map[string]liveTestData{
	"ec2": {
		baseURL:        DefaultBaseURL,
		requireSigned:  true,
		validCloudSpec: CloudSpec{"us-east-1", aws.Regions["us-east-1"].EC2Endpoint},
	},
	"canonistack": {
		baseURL:        "https://swift.canonistack.canonical.com/v1/AUTH_a48765cc0e864be980ee21ae26aaaed4/simplestreams/data",
		requireSigned:  false,
		validCloudSpec: CloudSpec{"lcy01", "https://keystone.canonistack.canonical.com:443/v2.0/"},
	},
}

func Test(t *testing.T) {
	if *live {
		if *vendor == "" {
			t.Fatal("missing vendor")
		}
		var ok bool
		var testData liveTestData
		if testData, ok = liveUrls[*vendor]; !ok {
			keys := reflect.ValueOf(liveUrls).MapKeys()
			t.Fatalf("Unknown vendor %s. Must be one of %s", *vendor, keys)
		}
		registerLiveSimpleStreamsTests(testData.baseURL, ImageConstraint{
			CloudSpec: testData.validCloudSpec,
			Series:    "quantal",
			Arches:    []string{"amd64"},
		}, testData.requireSigned)
	}
	registerSimpleStreamsTests()
	gc.Suite(&signingSuite{})
	gc.TestingT(t)
}

var testRoundTripper = &jujutest.ProxyRoundTripper{}

func init() {
	// Prepare mock http transport for overriding metadata and images output in tests.
	testRoundTripper.RegisterForScheme("test")
}

var imageData = map[string]string{
	"/streams/v1/index.json": `
		{
		 "index": {
		  "com.ubuntu.cloud:released:precise": {
		   "updated": "Wed, 01 May 2013 13:31:26 +0000",
		   "clouds": [
			{
			 "region": "us-east-1",
			 "endpoint": "https://ec2.us-east-1.amazonaws.com"
			}
		   ],
		   "cloudname": "aws",
		   "datatype": "image-ids",
		   "format": "products:1.0",
		   "products": [
			"com.ubuntu.cloud:server:12.04:amd64",
			"com.ubuntu.cloud:server:12.04:arm"
		   ],
		   "path": "streams/v1/image_metadata.json"
		  },
		  "com.ubuntu.cloud:released:raring": {
		   "updated": "Wed, 01 May 2013 13:31:26 +0000",
		   "clouds": [
			{
			 "region": "us-east-1",
			 "endpoint": "https://ec2.us-east-1.amazonaws.com"
			}
		   ],
		   "cloudname": "aws",
		   "datatype": "image-ids",
		   "format": "products:1.0",
		   "products": [
			"com.ubuntu.cloud:server:13.04:amd64"
		   ],
		   "path": "streams/v1/raring_metadata.json"
		  },
		  "com.ubuntu.cloud:released:download": {
		   "datatype": "image-downloads",
		   "path": "streams/v1/com.ubuntu.cloud:released:download.json",
		   "updated": "Wed, 01 May 2013 13:30:37 +0000",
		   "products": [
			"com.ubuntu.cloud:server:12.10:amd64",
			"com.ubuntu.cloud:server:13.04:amd64"
		   ],
		   "format": "products:1.0"
		  }
		 },
		 "updated": "Wed, 01 May 2013 13:31:26 +0000",
		 "format": "index:1.0"
		}
`,
	"/streams/v1/image_metadata.json": `
{
 "updated": "Wed, 01 May 2013 13:31:26 +0000",
 "content_id": "com.ubuntu.cloud:released:aws",
 "products": {
  "com.ubuntu.cloud:server:12.04:amd64": {
   "release": "precise",
   "version": "12.04",
   "arch": "amd64",
   "region": "au-east-1",
   "endpoint": "https://somewhere",
   "versions": {
    "20121218": {
     "region": "au-east-2",
     "endpoint": "https://somewhere-else",
     "items": {
      "usww1pe": {
       "root_store": "ebs",
       "virt": "pv",
       "id": "ami-26745463"
      },
      "usww2he": {
       "root_store": "ebs",
       "virt": "hvm",
       "id": "ami-442ea674",
       "region": "us-east-1",
       "endpoint": "https://ec2.us-east-1.amazonaws.com"
      },
      "usww3he": {
       "root_store": "ebs",
       "virt": "hvm",
       "crsn": "uswest3",
       "id": "ami-442ea675"
      }
     },
     "pubname": "ubuntu-precise-12.04-amd64-server-20121218",
     "label": "release"
    },
    "20111111": {
     "items": {
      "usww3pe": {
       "root_store": "ebs",
       "virt": "pv",
       "id": "ami-26745464"
      },
      "usww2pe": {
       "root_store": "instance",
       "virt": "pv",
       "id": "ami-442ea684",
       "region": "us-east-1",
       "endpoint": "https://ec2.us-east-1.amazonaws.com"
      }
     },
     "pubname": "ubuntu-precise-12.04-amd64-server-20111111",
     "label": "release"
    }
   }
  },
  "com.ubuntu.cloud:server:12.04:arm": {
   "release": "precise",
   "version": "12.04",
   "arch": "arm",
   "region": "us-east-1",
   "endpoint": "https://ec2.us-east-1.amazonaws.com",
   "versions": {
    "20121219": {
     "items": {
      "usww2he": {
       "root_store": "ebs",
       "virt": "pv",
       "id": "ami-442ea699"
      }
     },
     "pubname": "ubuntu-precise-12.04-arm-server-20121219",
     "label": "release"
    }
   }
  }
 },
 "_aliases": {
  "crsn": {
   "uswest3": {
    "region": "us-west-3",
    "endpoint": "https://ec2.us-west-3.amazonaws.com"
   }
  }
 },
 "format": "products:1.0"
}
`,
}

func registerSimpleStreamsTests() {
	gc.Suite(&simplestreamsSuite{
		liveSimplestreamsSuite: liveSimplestreamsSuite{
			baseURL:       "test:",
			requireSigned: false,
			validImageConstraint: ImageConstraint{
				CloudSpec: CloudSpec{
					Region:   "us-east-1",
					Endpoint: "https://ec2.us-east-1.amazonaws.com",
				},
				Series: "precise",
				Arches: []string{"amd64", "arm"},
			},
		},
	})
}

func registerLiveSimpleStreamsTests(baseURL string, validImageConstraint ImageConstraint, requireSigned bool) {
	gc.Suite(&liveSimplestreamsSuite{
		baseURL:              baseURL,
		requireSigned:        requireSigned,
		validImageConstraint: validImageConstraint,
	})
}

type simplestreamsSuite struct {
	liveSimplestreamsSuite
}

type liveSimplestreamsSuite struct {
	coretesting.LoggingSuite
	baseURL              string
	requireSigned        bool
	validImageConstraint ImageConstraint
}

func (s *liveSimplestreamsSuite) SetUpSuite(c *gc.C) {
	s.LoggingSuite.SetUpSuite(c)
}

func (s *liveSimplestreamsSuite) TearDownSuite(c *gc.C) {
	s.LoggingSuite.TearDownSuite(c)
}

func (s *simplestreamsSuite) SetUpSuite(c *gc.C) {
	s.liveSimplestreamsSuite.SetUpSuite(c)
	testRoundTripper.Sub = jujutest.NewCannedRoundTripper(
		imageData, map[string]int{"test://unauth": http.StatusUnauthorized})
}

func (s *simplestreamsSuite) TearDownSuite(c *gc.C) {
	testRoundTripper.Sub = nil
	s.liveSimplestreamsSuite.TearDownSuite(c)
}

const (
	index_v1   = "index:1.0"
	product_v1 = "products:1.0"
)

func (s *liveSimplestreamsSuite) indexPath() string {
	if s.requireSigned {
		return DefaultIndexPath + signedSuffix
	}
	return DefaultIndexPath + unsignedSuffix
}

func (s *liveSimplestreamsSuite) TestGetIndex(c *gc.C) {
	indexRef, err := getIndexWithFormat(s.baseURL, s.indexPath(), index_v1, s.requireSigned)
	c.Assert(err, gc.IsNil)
	c.Assert(indexRef.Format, gc.Equals, index_v1)
	c.Assert(indexRef.baseURL, gc.Equals, s.baseURL)
	c.Assert(len(indexRef.Indexes) > 0, gc.Equals, true)
}

func (s *liveSimplestreamsSuite) TestGetIndexWrongFormat(c *gc.C) {
	_, err := getIndexWithFormat(s.baseURL, s.indexPath(), "bad", s.requireSigned)
	c.Assert(err, gc.NotNil)
}

func (s *liveSimplestreamsSuite) TestGetImageIdsPathExists(c *gc.C) {
	indexRef, err := getIndexWithFormat(s.baseURL, s.indexPath(), index_v1, s.requireSigned)
	c.Assert(err, gc.IsNil)
	path, err := indexRef.getImageIdsPath(&s.validImageConstraint)
	c.Assert(err, gc.IsNil)
	c.Assert(path, gc.Not(gc.Equals), "")
}

func (s *liveSimplestreamsSuite) TestGetImageIdsPathInvalidCloudSpec(c *gc.C) {
	indexRef, err := getIndexWithFormat(s.baseURL, s.indexPath(), index_v1, s.requireSigned)
	c.Assert(err, gc.IsNil)
	ic := ImageConstraint{
		CloudSpec: CloudSpec{"bad", "spec"},
	}
	_, err = indexRef.getImageIdsPath(&ic)
	c.Assert(err, gc.NotNil)
}

func (s *liveSimplestreamsSuite) TestGetImageIdsPathInvalidProductSpec(c *gc.C) {
	indexRef, err := getIndexWithFormat(s.baseURL, s.indexPath(), index_v1, s.requireSigned)
	c.Assert(err, gc.IsNil)
	ic := ImageConstraint{
		CloudSpec: s.validImageConstraint.CloudSpec,
		Series:    "precise",
		Arches:    []string{"bad"},
		Stream:    "spec",
	}
	_, err = indexRef.getImageIdsPath(&ic)
	c.Assert(err, gc.NotNil)
}

func (s *simplestreamsSuite) TestGetImageIdsPath(c *gc.C) {
	indexRef, err := getIndexWithFormat(s.baseURL, s.indexPath(), index_v1, s.requireSigned)
	c.Assert(err, gc.IsNil)
	path, err := indexRef.getImageIdsPath(&s.validImageConstraint)
	c.Assert(err, gc.IsNil)
	c.Assert(path, gc.Equals, "streams/v1/image_metadata.json")
}

func (*simplestreamsSuite) TestExtractCatalogsForProductsAcceptsNil(c *gc.C) {
	empty := cloudImageMetadata{}
	c.Check(empty.extractCatalogsForProducts(nil), gc.HasLen, 0)
}

func (*simplestreamsSuite) TestExtractCatalogsForProductsReturnsMatch(c *gc.C) {
	metadata := cloudImageMetadata{
		Products: map[string]imageMetadataCatalog{
			"foo": {},
		},
	}
	c.Check(
		metadata.extractCatalogsForProducts([]string{"foo"}),
		gc.DeepEquals,
		[]imageMetadataCatalog{metadata.Products["foo"]})
}

func (*simplestreamsSuite) TestExtractCatalogsForProductsIgnoresNonMatches(c *gc.C) {
	metadata := cloudImageMetadata{
		Products: map[string]imageMetadataCatalog{
			"one-product": {},
		},
	}
	absentProducts := []string{"another-product"}
	c.Check(metadata.extractCatalogsForProducts(absentProducts), gc.HasLen, 0)
}

func (*simplestreamsSuite) TestExtractCatalogsForProductsPreservesOrder(c *gc.C) {
	products := map[string]imageMetadataCatalog{
		"1": {},
		"2": {},
		"3": {},
		"4": {},
	}

	metadata := cloudImageMetadata{Products: products}

	c.Check(
		metadata.extractCatalogsForProducts([]string{"1", "3", "4", "2"}),
		gc.DeepEquals,
		[]imageMetadataCatalog{
			products["1"],
			products["3"],
			products["4"],
			products["2"],
		})
}

func (*simplestreamsSuite) TestExtractIndexesAcceptsNil(c *gc.C) {
	ind := indices{}
	c.Check(ind.extractIndexes(), gc.HasLen, 0)
}

func (*simplestreamsSuite) TestExtractIndexesReturnsIndex(c *gc.C) {
	metadata := indexMetadata{}
	ind := indices{Indexes: map[string]*indexMetadata{"foo": &metadata}}
	c.Check(ind.extractIndexes(), gc.DeepEquals, indexMetadataSlice{&metadata})
}

func (*simplestreamsSuite) TestExtractIndexesReturnsAllIndexes(c *gc.C) {
	ind := indices{
		Indexes: map[string]*indexMetadata{
			"foo": {},
			"bar": {},
		},
	}

	array := ind.extractIndexes()

	c.Assert(array, gc.HasLen, len(ind.Indexes))
	c.Check(array[0], gc.NotNil)
	c.Check(array[1], gc.NotNil)
	c.Check(array[0], gc.Not(gc.Equals), array[1])
	c.Check(
		(array[0] == ind.Indexes["foo"]),
		gc.Not(gc.Equals),
		(array[1] == ind.Indexes["foo"]))
	c.Check(
		(array[0] == ind.Indexes["bar"]),
		gc.Not(gc.Equals),
		(array[1] == ind.Indexes["bar"]))
}

func (*simplestreamsSuite) TestHasCloudAcceptsNil(c *gc.C) {
	metadata := indexMetadata{Clouds: nil}
	c.Check(metadata.hasCloud(CloudSpec{}), gc.Equals, false)
}

func (*simplestreamsSuite) TestHasCloudFindsMatch(c *gc.C) {
	metadata := indexMetadata{
		Clouds: []CloudSpec{
			{Region: "r1", Endpoint: "http://e1"},
			{Region: "r2", Endpoint: "http://e2"},
		},
	}
	c.Check(metadata.hasCloud(metadata.Clouds[1]), gc.Equals, true)
}

func (*simplestreamsSuite) TestHasCloudReturnsFalseIfCloudsDoNotMatch(c *gc.C) {
	metadata := indexMetadata{
		Clouds: []CloudSpec{
			{Region: "r1", Endpoint: "http://e1"},
			{Region: "r2", Endpoint: "http://e2"},
		},
	}
	otherCloud := CloudSpec{Region: "r9", Endpoint: "http://e9"}
	c.Check(metadata.hasCloud(otherCloud), gc.Equals, false)
}

func (*simplestreamsSuite) TestHasCloudRequiresIdenticalRegion(c *gc.C) {
	metadata := indexMetadata{
		Clouds: []CloudSpec{
			{Region: "around", Endpoint: "http://nearby"},
		},
	}
	similarCloud := metadata.Clouds[0]
	similarCloud.Region = "elsewhere"
	c.Assert(similarCloud, gc.Not(gc.Equals), metadata.Clouds[0])

	c.Check(metadata.hasCloud(similarCloud), gc.Equals, false)
}

func (*simplestreamsSuite) TestHasCloudRequiresIdenticalEndpoint(c *gc.C) {
	metadata := indexMetadata{
		Clouds: []CloudSpec{
			{Region: "around", Endpoint: "http://nearby"},
		},
	}
	similarCloud := metadata.Clouds[0]
	similarCloud.Endpoint = "http://far"
	c.Assert(similarCloud, gc.Not(gc.Equals), metadata.Clouds[0])

	c.Check(metadata.hasCloud(similarCloud), gc.Equals, false)
}

func (*simplestreamsSuite) TestHasProductAcceptsNils(c *gc.C) {
	metadata := indexMetadata{}
	c.Check(metadata.hasProduct(nil), gc.Equals, false)
}

func (*simplestreamsSuite) TestHasProductFindsMatchingProduct(c *gc.C) {
	metadata := indexMetadata{ProductIds: []string{"x", "y", "z"}}
	c.Check(
		metadata.hasProduct([]string{"a", "b", metadata.ProductIds[1]}),
		gc.Equals,
		true)
}

func (*simplestreamsSuite) TestHasProductReturnsFalseIfProductsDoNotMatch(c *gc.C) {
	metadata := indexMetadata{ProductIds: []string{"x", "y", "z"}}
	c.Check(metadata.hasProduct([]string{"a", "b", "c"}), gc.Equals, false)
}

func (*simplestreamsSuite) TestFilterReturnsNothingForEmptyArray(c *gc.C) {
	empty := indexMetadataSlice{}
	c.Check(
		empty.filter(func(*indexMetadata) bool { return true }),
		gc.HasLen,
		0)
}

func (*simplestreamsSuite) TestFilterRemovesNonMatches(c *gc.C) {
	array := indexMetadataSlice{&indexMetadata{}}
	c.Check(
		array.filter(func(*indexMetadata) bool { return false }),
		gc.HasLen,
		0)
}

func (*simplestreamsSuite) TestFilterIncludesMatches(c *gc.C) {
	metadata := indexMetadata{}
	array := indexMetadataSlice{&metadata}
	c.Check(
		array.filter(func(*indexMetadata) bool { return true }),
		gc.DeepEquals,
		indexMetadataSlice{&metadata})
}

func (*simplestreamsSuite) TestFilterLeavesOriginalUnchanged(c *gc.C) {
	item1 := indexMetadata{CloudName: "aws"}
	item2 := indexMetadata{CloudName: "openstack"}
	array := indexMetadataSlice{&item1, &item2}

	result := array.filter(func(metadata *indexMetadata) bool {
		return metadata.CloudName == "aws"
	})
	// This exercises both the "leave out" and the "include" code paths.
	c.Assert(result, gc.HasLen, 1)

	// The original, however, has not changed.
	c.Assert(array, gc.HasLen, 2)
	c.Check(array, gc.DeepEquals, indexMetadataSlice{&item1, &item2})
}

func (*simplestreamsSuite) TestFilterPreservesOrder(c *gc.C) {
	array := indexMetadataSlice{
		&indexMetadata{CloudName: "aws"},
		&indexMetadata{CloudName: "maas"},
		&indexMetadata{CloudName: "openstack"},
	}

	c.Check(
		array.filter(func(metadata *indexMetadata) bool { return true }),
		gc.DeepEquals,
		array)
}

func (*simplestreamsSuite) TestFilterCombinesMatchesAndNonMatches(c *gc.C) {
	array := indexMetadataSlice{
		&indexMetadata{Format: "1.0"},
		&indexMetadata{Format: "1.1"},
		&indexMetadata{Format: "2.0"},
		&indexMetadata{Format: "2.1"},
	}

	dotOFormats := array.filter(func(metadata *indexMetadata) bool {
		return strings.HasSuffix(metadata.Format, ".0")
	})

	c.Check(dotOFormats, gc.DeepEquals, indexMetadataSlice{array[0], array[2]})
}

func (s *simplestreamsSuite) TestFetchNoSignedMetadata(c *gc.C) {
	im, err := Fetch([]string{s.baseURL}, DefaultIndexPath, &s.validImageConstraint, true)
	c.Assert(err, gc.IsNil)
	c.Assert(im, gc.HasLen, 0)
}

func (s *liveSimplestreamsSuite) assertGetMetadata(c *gc.C) *cloudImageMetadata {
	indexRef, err := getIndexWithFormat(s.baseURL, s.indexPath(), index_v1, s.requireSigned)
	c.Assert(err, gc.IsNil)
	metadata, err := indexRef.getCloudMetadataWithFormat(&s.validImageConstraint, product_v1, s.requireSigned)
	c.Assert(err, gc.IsNil)
	c.Assert(metadata.Format, gc.Equals, product_v1)
	c.Assert(len(metadata.Products) > 0, gc.Equals, true)
	return metadata
}

func (s *liveSimplestreamsSuite) TestGetCloudMetadataWithFormat(c *gc.C) {
	s.assertGetMetadata(c)
}

func (s *liveSimplestreamsSuite) TestFetchExists(c *gc.C) {
	im, err := Fetch([]string{s.baseURL}, DefaultIndexPath, &s.validImageConstraint, s.requireSigned)
	c.Assert(err, gc.IsNil)
	c.Assert(len(im) > 0, gc.Equals, true)
}

func (s *liveSimplestreamsSuite) TestFetchFirstURLNotFound(c *gc.C) {
	im, err := Fetch([]string{"test://notfound", s.baseURL}, DefaultIndexPath, &s.validImageConstraint, s.requireSigned)
	c.Assert(err, gc.IsNil)
	c.Assert(len(im) > 0, gc.Equals, true)
}

func (s *liveSimplestreamsSuite) TestFetchFirstURLUnauthorised(c *gc.C) {
	im, err := Fetch([]string{"test://unauth", s.baseURL}, DefaultIndexPath, &s.validImageConstraint, s.requireSigned)
	c.Assert(err, gc.IsNil)
	c.Assert(len(im) > 0, gc.Equals, true)
}

func (s *liveSimplestreamsSuite) assertGetImageCollections(c *gc.C, version string) *imageCollection {
	metadata := s.assertGetMetadata(c)
	metadataCatalog := metadata.Products["com.ubuntu.cloud:server:12.04:amd64"]
	ic := metadataCatalog.Images[version]
	return ic
}

func (s *simplestreamsSuite) TestMetadataCatalog(c *gc.C) {
	metadata := s.assertGetMetadata(c)
	c.Check(len(metadata.Products), gc.Equals, 2)
	c.Check(len(metadata.Aliases), gc.Equals, 1)
	metadataCatalog := metadata.Products["com.ubuntu.cloud:server:12.04:amd64"]
	c.Check(len(metadataCatalog.Images), gc.Equals, 2)
	c.Check(metadataCatalog.Series, gc.Equals, "precise")
	c.Check(metadataCatalog.Version, gc.Equals, "12.04")
	c.Check(metadataCatalog.Arch, gc.Equals, "amd64")
	c.Check(metadataCatalog.RegionName, gc.Equals, "au-east-1")
	c.Check(metadataCatalog.Endpoint, gc.Equals, "https://somewhere")
	c.Check(len(metadataCatalog.Images) > 0, gc.Equals, true)
}

func (s *simplestreamsSuite) TestImageCollection(c *gc.C) {
	ic := s.assertGetImageCollections(c, "20121218")
	c.Check(ic.RegionName, gc.Equals, "au-east-2")
	c.Check(ic.Endpoint, gc.Equals, "https://somewhere-else")
	c.Assert(len(ic.Images) > 0, gc.Equals, true)
	im := ic.Images["usww2he"]
	c.Check(im.Id, gc.Equals, "ami-442ea674")
	c.Check(im.Storage, gc.Equals, "ebs")
	c.Check(im.VType, gc.Equals, "hvm")
	c.Check(im.RegionName, gc.Equals, "us-east-1")
	c.Check(im.Endpoint, gc.Equals, "https://ec2.us-east-1.amazonaws.com")
}

func (s *simplestreamsSuite) TestImageMetadataDenormalisationFromCollection(c *gc.C) {
	ic := s.assertGetImageCollections(c, "20121218")
	im := ic.Images["usww1pe"]
	c.Check(im.RegionName, gc.Equals, ic.RegionName)
	c.Check(im.Endpoint, gc.Equals, ic.Endpoint)
}

func (s *simplestreamsSuite) TestImageMetadataDenormalisationFromCatalog(c *gc.C) {
	metadata := s.assertGetMetadata(c)
	metadataCatalog := metadata.Products["com.ubuntu.cloud:server:12.04:amd64"]
	ic := metadataCatalog.Images["20111111"]
	im := ic.Images["usww3pe"]
	c.Check(im.RegionName, gc.Equals, metadataCatalog.RegionName)
	c.Check(im.Endpoint, gc.Equals, metadataCatalog.Endpoint)
}

func (s *simplestreamsSuite) TestImageMetadataDealiasing(c *gc.C) {
	metadata := s.assertGetMetadata(c)
	metadataCatalog := metadata.Products["com.ubuntu.cloud:server:12.04:amd64"]
	ic := metadataCatalog.Images["20121218"]
	im := ic.Images["usww3he"]
	c.Check(im.RegionName, gc.Equals, "us-west-3")
	c.Check(im.Endpoint, gc.Equals, "https://ec2.us-west-3.amazonaws.com")
}

type productSpecSuite struct{}

var _ = gc.Suite(&productSpecSuite{})

func (s *productSpecSuite) TestIdWithDefaultStream(c *gc.C) {
	imageConstraint := ImageConstraint{
		Series: "precise",
		Arches: []string{"amd64"},
	}
	ids, err := imageConstraint.Ids()
	c.Assert(err, gc.IsNil)
	c.Assert(ids, gc.DeepEquals, []string{"com.ubuntu.cloud:server:12.04:amd64"})
}

func (s *productSpecSuite) TestId(c *gc.C) {
	imageConstraint := ImageConstraint{
		Series: "precise",
		Arches: []string{"amd64"},
		Stream: "daily",
	}
	ids, err := imageConstraint.Ids()
	c.Assert(err, gc.IsNil)
	c.Assert(ids, gc.DeepEquals, []string{"com.ubuntu.cloud.daily:server:12.04:amd64"})
}

func (s *productSpecSuite) TestIdMultiArch(c *gc.C) {
	imageConstraint := ImageConstraint{
		Series: "precise",
		Arches: []string{"amd64", "i386"},
		Stream: "daily",
	}
	ids, err := imageConstraint.Ids()
	c.Assert(err, gc.IsNil)
	c.Assert(ids, gc.DeepEquals, []string{
		"com.ubuntu.cloud.daily:server:12.04:amd64",
		"com.ubuntu.cloud.daily:server:12.04:i386"})
}

func (s *productSpecSuite) TestIdWithNonDefaultRelease(c *gc.C) {
	imageConstraint := ImageConstraint{
		Series: "lucid",
		Arches: []string{"amd64"},
		Stream: "daily",
	}
	ids, err := imageConstraint.Ids()
	if err != nil && err.Error() == `invalid series "lucid"` {
		c.Fatalf(`Unable to lookup series "lucid", you may need to: apt-get install distro-info`)
	}
	c.Assert(err, gc.IsNil)
	c.Assert(ids, gc.DeepEquals, []string{"com.ubuntu.cloud.daily:server:10.04:amd64"})
}

var fetchTests = []struct {
	region string
	series string
	arches []string
	images []*ImageMetadata
}{
	{
		region: "us-east-1",
		series: "precise",
		arches: []string{"amd64", "arm"},
		images: []*ImageMetadata{
			{
				Id:         "ami-442ea674",
				VType:      "hvm",
				Arch:       "amd64",
				RegionName: "us-east-1",
				Endpoint:   "https://ec2.us-east-1.amazonaws.com",
				Storage:    "ebs",
			},
			{
				Id:         "ami-442ea684",
				VType:      "pv",
				Arch:       "amd64",
				RegionName: "us-east-1",
				Endpoint:   "https://ec2.us-east-1.amazonaws.com",
				Storage:    "instance",
			},
			{
				Id:         "ami-442ea699",
				VType:      "pv",
				Arch:       "arm",
				RegionName: "us-east-1",
				Endpoint:   "https://ec2.us-east-1.amazonaws.com",
				Storage:    "ebs",
			},
		},
	},
	{
		region: "us-east-1",
		series: "precise",
		arches: []string{"amd64"},
		images: []*ImageMetadata{
			{
				Id:         "ami-442ea674",
				VType:      "hvm",
				Arch:       "amd64",
				RegionName: "us-east-1",
				Endpoint:   "https://ec2.us-east-1.amazonaws.com",
				Storage:    "ebs",
			},
			{
				Id:         "ami-442ea684",
				VType:      "pv",
				Arch:       "amd64",
				RegionName: "us-east-1",
				Endpoint:   "https://ec2.us-east-1.amazonaws.com",
				Storage:    "instance",
			},
		},
	},
	{
		region: "us-east-1",
		series: "precise",
		arches: []string{"arm"},
		images: []*ImageMetadata{
			{
				Id:         "ami-442ea699",
				VType:      "pv",
				Arch:       "arm",
				RegionName: "us-east-1",
				Endpoint:   "https://ec2.us-east-1.amazonaws.com",
				Storage:    "ebs",
			},
		},
	},
	{
		region: "us-east-1",
		series: "precise",
		arches: []string{"amd64"},
		images: []*ImageMetadata{
			{
				Id:         "ami-442ea674",
				VType:      "hvm",
				Arch:       "amd64",
				RegionName: "us-east-1",
				Endpoint:   "https://ec2.us-east-1.amazonaws.com",
				Storage:    "ebs",
			},
			{
				Id:         "ami-442ea684",
				VType:      "pv",
				Arch:       "amd64",
				RegionName: "us-east-1",
				Endpoint:   "https://ec2.us-east-1.amazonaws.com",
				Storage:    "instance",
			},
		},
	},
}

func (s *simplestreamsSuite) TestFetch(c *gc.C) {
	for i, t := range fetchTests {
		c.Logf("test %d", i)
		imageConstraint := ImageConstraint{
			CloudSpec: CloudSpec{t.region, "https://ec2.us-east-1.amazonaws.com"},
			Series:    "precise",
			Arches:    t.arches,
		}
		images, err := Fetch([]string{s.baseURL}, DefaultIndexPath, &imageConstraint, s.requireSigned)
		if !c.Check(err, gc.IsNil) {
			continue
		}
		c.Check(images, gc.DeepEquals, t.images)
	}
}

var testSigningKey = `-----BEGIN PGP PRIVATE KEY BLOCK-----
Version: GnuPG v1.4.10 (GNU/Linux)

lQHYBE2rFNoBBADFwqWQIW/DSqcB4yCQqnAFTJ27qS5AnB46ccAdw3u4Greeu3Bp
idpoHdjULy7zSKlwR1EA873dO/k/e11Ml3dlAFUinWeejWaK2ugFP6JjiieSsrKn
vWNicdCS4HTWn0X4sjl0ZiAygw6GNhqEQ3cpLeL0g8E9hnYzJKQ0LWJa0QARAQAB
AAP/TB81EIo2VYNmTq0pK1ZXwUpxCrvAAIG3hwKjEzHcbQznsjNvPUihZ+NZQ6+X
0HCfPAdPkGDCLCb6NavcSW+iNnLTrdDnSI6+3BbIONqWWdRDYJhqZCkqmG6zqSfL
IdkJgCw94taUg5BWP/AAeQrhzjChvpMQTVKQL5mnuZbUCeMCAN5qrYMP2S9iKdnk
VANIFj7656ARKt/nf4CBzxcpHTyB8+d2CtPDKCmlJP6vL8t58Jmih+kHJMvC0dzn
gr5f5+sCAOOe5gt9e0am7AvQWhdbHVfJU0TQJx+m2OiCJAqGTB1nvtBLHdJnfdC9
TnXXQ6ZXibqLyBies/xeY2sCKL5qtTMCAKnX9+9d/5yQxRyrQUHt1NYhaXZnJbHx
q4ytu0eWz+5i68IYUSK69jJ1NWPM0T6SkqpB3KCAIv68VFm9PxqG1KmhSrQIVGVz
dCBLZXmIuAQTAQIAIgUCTasU2gIbAwYLCQgHAwIGFQgCCQoLBBYCAwECHgECF4AA
CgkQO9o98PRieSoLhgQAkLEZex02Qt7vGhZzMwuN0R22w3VwyYyjBx+fM3JFETy1
ut4xcLJoJfIaF5ZS38UplgakHG0FQ+b49i8dMij0aZmDqGxrew1m4kBfjXw9B/v+
eIqpODryb6cOSwyQFH0lQkXC040pjq9YqDsO5w0WYNXYKDnzRV0p4H1pweo2VDid
AdgETasU2gEEAN46UPeWRqKHvA99arOxee38fBt2CI08iiWyI8T3J6ivtFGixSqV
bRcPxYO/qLpVe5l84Nb3X71GfVXlc9hyv7CD6tcowL59hg1E/DC5ydI8K8iEpUmK
/UnHdIY5h8/kqgGxkY/T/hgp5fRQgW1ZoZxLajVlMRZ8W4tFtT0DeA+JABEBAAEA
A/0bE1jaaZKj6ndqcw86jd+QtD1SF+Cf21CWRNeLKnUds4FRRvclzTyUMuWPkUeX
TaNNsUOFqBsf6QQ2oHUBBK4VCHffHCW4ZEX2cd6umz7mpHW6XzN4DECEzOVksXtc
lUC1j4UB91DC/RNQqwX1IV2QLSwssVotPMPqhOi0ZLNY7wIA3n7DWKInxYZZ4K+6
rQ+POsz6brEoRHwr8x6XlHenq1Oki855pSa1yXIARoTrSJkBtn5oI+f8AzrnN0BN
oyeQAwIA/7E++3HDi5aweWrViiul9cd3rcsS0dEnksPhvS0ozCJiHsq/6GFmy7J8
QSHZPteedBnZyNp5jR+H7cIfVN3KgwH/Skq4PsuPhDq5TKK6i8Pc1WW8MA6DXTdU
nLkX7RGmMwjC0DBf7KWAlPjFaONAX3a8ndnz//fy1q7u2l9AZwrj1qa1iJ8EGAEC
AAkFAk2rFNoCGwwACgkQO9o98PRieSo2/QP/WTzr4ioINVsvN1akKuekmEMI3LAp
BfHwatufxxP1U+3Si/6YIk7kuPB9Hs+pRqCXzbvPRrI8NHZBmc8qIGthishdCYad
AHcVnXjtxrULkQFGbGvhKURLvS9WnzD/m1K2zzwxzkPTzT9/Yf06O6Mal5AdugPL
VrM0m72/jnpKo04=
=zNCn
-----END PGP PRIVATE KEY BLOCK-----
`

var validClearsignInput = `
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA1

Hello world
line 2
`

var invalidClearsignInput = `
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA1

Invalid
`

var testSig = `-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1.4.10 (GNU/Linux)

iJwEAQECAAYFAk8kMuEACgkQO9o98PRieSpMsAQAhmY/vwmNpflrPgmfWsYhk5O8
pjnBUzZwqTDoDeINjZEoPDSpQAHGhjFjgaDx/Gj4fAl0dM4D0wuUEBb6QOrwflog
2A2k9kfSOMOtk0IH/H5VuFN1Mie9L/erYXjTQIptv9t9J7NoRBMU0QOOaFU0JaO9
MyTpno24AjIAGb+mH1U=
=hIJ6
-----END PGP SIGNATURE-----
`

var origKey = simpleStreamSigningKey

type signingSuite struct{}

func (s *signingSuite) SetUpSuite(c *gc.C) {
	simpleStreamSigningKey = testSigningKey
}

func (s *signingSuite) TearDownSuite(c *gc.C) {
	simpleStreamSigningKey = origKey
}

func (s *signingSuite) TestDecodeCheckValidSignature(c *gc.C) {
	r := bytes.NewReader([]byte(validClearsignInput + testSig))
	txt, err := DecodeCheckSignature(r)
	c.Assert(err, gc.IsNil)
	c.Assert(txt, gc.DeepEquals, []byte("Hello world\nline 2\n"))
}

func (s *signingSuite) TestDecodeCheckInvalidSignature(c *gc.C) {
	r := bytes.NewReader([]byte(invalidClearsignInput + testSig))
	_, err := DecodeCheckSignature(r)
	c.Assert(err, gc.Not(gc.IsNil))
	_, ok := err.(*NotPGPSignedError)
	c.Assert(ok, gc.Equals, false)
}

func (s *signingSuite) TestDecodeCheckMissingSignature(c *gc.C) {
	r := bytes.NewReader([]byte("foo"))
	_, err := DecodeCheckSignature(r)
	_, ok := err.(*NotPGPSignedError)
	c.Assert(ok, gc.Equals, true)
}
