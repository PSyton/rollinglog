package rollinglog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	l := New(WithLogFile("somelog.log"))

	assert.Equal(t, "somelog.log", l.filename)

	UseCompression(l)
	assert.True(t, l.compress)

	WithMaxBytes(1000)(l)
	assert.Equal(t, uint64(1000), l.sizeLimit)

	WithMaxAge(5)(l)
	assert.Equal(t, 5, l.backupsDaysLimit)

	WithMaxBackups(2)(l)
	assert.Equal(t, 2, l.backupsCountLimit)

	UseLocaltime(l)
	assert.True(t, l.localtime)

	Options(WithMaxBackups(0), WithMaxAge(20))(l)
	assert.Equal(t, 0, l.backupsCountLimit)
	assert.Equal(t, 20, l.backupsDaysLimit)
}
