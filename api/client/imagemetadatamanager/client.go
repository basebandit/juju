// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package imagemetadatamanager

import (
	"github.com/juju/errors"

	"github.com/juju/juju/api/base"
	coreseries "github.com/juju/juju/core/series"
	"github.com/juju/juju/rpc/params"
)

// Client provides access to cloud image metadata.
// It is used to find, save and update image metadata.
type Client struct {
	base.ClientFacade
	facade base.FacadeCaller
}

// NewClient returns a new metadata client.
func NewClient(st base.APICallCloser) *Client {
	frontend, backend := base.NewClientFacade(st, "ImageMetadataManager")
	return &Client{ClientFacade: frontend, facade: backend}
}

// List returns image metadata that matches filter.
// Empty filter will return all image metadata.
func (c *Client) List(
	stream, region string,
	series, arches []string,
	virtType, rootStorageType string,
) ([]params.CloudImageMetadata, error) {
	versions := make([]string, len(series))
	for i, s := range series {
		vers, err := coreseries.SeriesVersion(s)
		if err != nil {
			return nil, errors.Trace(err)
		}
		versions[i] = vers
	}
	in := params.ImageMetadataFilter{
		Region:          region,
		Series:          series,
		Versions:        versions,
		Arches:          arches,
		Stream:          stream,
		VirtType:        virtType,
		RootStorageType: rootStorageType,
	}
	out := params.ListCloudImageMetadataResult{}
	err := c.facade.FacadeCall("List", in, &out)
	return out.Result, err
}

// Save saves specified image metadata.
// Supports bulk saves for scenarios like cloud image metadata caching at bootstrap.
func (c *Client) Save(metadata []params.CloudImageMetadata) error {
	in := params.MetadataSaveParams{
		Metadata: []params.CloudImageMetadataList{{metadata}},
	}
	out := params.ErrorResults{}
	err := c.facade.FacadeCall("Save", in, &out)
	if err != nil {
		return errors.Trace(err)
	}
	if len(out.Results) != 1 {
		return errors.Errorf("exected 1 result, got %d", len(out.Results))
	}
	if out.Results[0].Error != nil {
		return errors.Trace(out.Results[0].Error)
	}
	return nil
}

// Delete removes image metadata for given image id from stored metadata.
func (c *Client) Delete(imageId string) error {
	in := params.MetadataImageIds{[]string{imageId}}
	out := params.ErrorResults{}
	err := c.facade.FacadeCall("Delete", in, &out)
	if err != nil {
		return errors.Trace(err)
	}

	result := out.Results
	if len(result) != 1 {
		return errors.Errorf("expected to find one result for image id %q but found %d", imageId, len(result))
	}

	theOne := result[0]
	if theOne.Error != nil {
		return errors.Trace(theOne.Error)
	}
	return nil
}
