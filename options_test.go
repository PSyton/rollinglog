package rollinglog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	l := New(LogFile("somelog.log"))

	assert.Equal(t, "somelog.log", l.filename)

	Compression(true)(l)
	assert.True(t, l.compress)

	Compression(false)(l)
	assert.False(t, l.compress)

	MaxBytes(1000)(l)
	assert.Equal(t, uint64(1000), l.sizeLimit)

	MaxAge(5)(l)
	assert.Equal(t, 5, l.backupsDaysLimit)

	MaxBackups(2)(l)
	assert.Equal(t, 2, l.backupsCountLimit)

	Localtime(true)(l)
	assert.True(t, l.localtime)

	Localtime(false)(l)
	assert.False(t, l.localtime)
}
