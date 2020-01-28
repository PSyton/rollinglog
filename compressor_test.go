package rollinglog

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompress(t *testing.T) {
	dir := makeTempDir("TestNewCompress", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)
	data := []byte("somedatasomedatasomedatasomedatasomedatasomedatasomedata")

	require.NoError(t, ioutil.WriteFile(lf, data, 0644))

	c := newCompressor(lf)

	assert.True(t, strings.HasPrefix(c.destFile, c.sourceFile))
	assert.True(t, strings.HasSuffix(c.destFile, compressSuffix))

	require.NoError(t, c.Compress())

	info, err := os.Stat(c.sourceFile)
	assert.True(t, err != nil && os.IsNotExist(err), "source not deleted")

	info, err = os.Stat(c.destFile)
	assert.True(t, err == nil && !info.IsDir(), "dest not found")
	assert.Greater(t, info.Size(), int64(0), "zero compressed file")

	w := &bytes.Buffer{}
	gz := gzip.NewWriter(w)
	_, err = io.Copy(gz, bytes.NewReader(data))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	assert.Equal(t, int64(len(w.Bytes())), info.Size())
}

func TestCompressNotExisting(t *testing.T) {
	dir := makeTempDir("TestCompressNotExisting", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)

	c := newCompressor(lf)
	require.Error(t, c.Compress())

	_, err := os.Stat(c.destFile)
	assert.True(t, err != nil && os.IsNotExist(err), "dest created")
}
