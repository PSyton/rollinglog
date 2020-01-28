package rollinglog

import (
	"compress/gzip"
	"io"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

type compressor struct {
	destFile   string
	sourceFile string

	errors *multierror.Error

	src           *os.File
	dst           *os.File
	fileForRemove string
}

func newCompressor(aSource string) *compressor {
	return &compressor{
		sourceFile: aSource,
		destFile:   aSource + compressSuffix,
		errors:     new(multierror.Error),
	}
}

func (c *compressor) Compress() (err error) {
	if c.src, err = os.Open(c.sourceFile); err != nil {
		c.errors = multierror.Append(c.errors, errors.Wrap(err, "Failed open log for compress"))
		return c.finish()
	}

	if c.dst, err = os.OpenFile(c.destFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileMode); err != nil {
		c.errors = multierror.Append(c.errors, errors.Wrap(err, "Failed to create compressed log"))
		return c.finish()
	}

	gz := gzip.NewWriter(c.dst)

	if _, err = io.Copy(gz, c.src); err != nil {
		c.fileForRemove = c.destFile
		c.errors = multierror.Append(c.errors, errors.Wrap(err, "Failed to write compressed log file"))
		return c.finish()
	}

	if err = gz.Close(); err != nil {
		c.fileForRemove = c.destFile
		c.errors = multierror.Append(errors.Wrapf(err, "Failed to close gz writer for %s", c.destFile))
		return c.finish()
	}

	c.fileForRemove = c.sourceFile
	return c.finish()
}

func (c *compressor) finish() error {
	if c.src != nil {
		if e := c.src.Close(); e != nil {
			c.errors = multierror.Append(errors.Wrapf(e, "Failed to close %s", c.sourceFile))
		}
	}

	if c.dst != nil {
		if e := c.dst.Close(); e != nil {
			c.errors = multierror.Append(errors.Wrapf(e, "Failed to close %s", c.destFile))
		}
	}

	if c.fileForRemove != "" {
		if e := os.Remove(c.fileForRemove); e != nil {
			c.errors = multierror.Append(errors.Wrapf(e, "Failed to remove %s", c.fileForRemove))
		}
	}

	return c.errors.ErrorOrNil()
}
