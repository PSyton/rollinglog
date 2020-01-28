package rollinglog

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaults(t *testing.T) {
	l := New()

	assert.Equal(t, uint64(0), l.sizeLimit)
	assert.Equal(t, 0, l.backupsDaysLimit)
	assert.Equal(t, 0, l.backupsCountLimit)
	assert.False(t, l.compress)
	assert.False(t, l.localtime)
}

type testEH struct{}

func (eh *testEH) OnError(aError error) {
}

func TestWidthErrorHandler(t *testing.T) {
	l := New()

	def := reflect.ValueOf(defaultErrorHandler)

	assert.Equal(t, def, reflect.ValueOf(l.errHandler))

	var eh ErrHandler = func(error) {}

	l = New(ErrorHandler(eh))
	assert.Equal(t, reflect.ValueOf(eh), reflect.ValueOf(l.errHandler))

	l = New(ErrorHandler(nil))
	assert.Equal(t, def, reflect.ValueOf(l.errHandler))
}

func TestSizeExceeded(t *testing.T) {
	assert.False(t, sizeExceeded(100, 0))
}

func TestDefaultFilename(t *testing.T) {
	l := New()
	expected := filepath.Join(os.TempDir(), filepath.Base(os.Args[0])+"-rollinglog.log")
	assert.Equal(t, expected, l.filename)
}

func TestAutoRotate(t *testing.T) {
	dir := makeTempDir("TestAutoRotate", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)
	l := New(LogFile(lf), MaxBytes(10), MaxBackups(5), MaxAge(7))
	defer l.Close()

	b := []byte("123456789")
	n, err := l.Write(b)
	require.NoError(t, err)
	assert.Equal(t, len(b), n)

	existsWithContent(lf, b, t)
	fileCount(dir, 1, t)

	b = []byte("987654321")

	n, err = l.Write(b)
	require.NoError(t, err)
	assert.Equal(t, len(b), n)

	existsWithContent(lf, b, t)
	fileCount(dir, 2, t)
}

func TestFirstWriteWithRotate(t *testing.T) {
	dir := makeTempDir("TestFirstWriteWithRotate", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)
	l := New(LogFile(lf), MaxBytes(10), MaxBackups(5), MaxAge(7))

	b := []byte("123456789")
	n, err := l.Write(b)
	require.NoError(t, err)
	assert.Equal(t, len(b), n)
	require.NoError(t, l.Close())

	existsWithContent(lf, b, t)
	fileCount(dir, 1, t)

	l = New(LogFile(lf), MaxBytes(10), MaxBackups(5), MaxAge(7))

	b = []byte("987654321")

	n, err = l.Write(b)
	require.NoError(t, err)
	assert.Equal(t, len(b), n)

	existsWithContent(lf, b, t)
	fileCount(dir, 2, t)
	require.NoError(t, l.Close())
}

func TestSplitFilename(t *testing.T) {
	tests := []struct {
		filename   string
		wantPrefix string
		wantSuffix string
	}{
		{"test.log", "test.", ".log"},
		{"test", "test.", ""},
		{".test", ".", ".test"},
	}
	for _, test := range tests {
		p, s := splitFilename(test.filename)
		assert.Equal(t, test.wantPrefix, p)
		assert.Equal(t, test.wantSuffix, s)
	}
}

func TestTimeFormFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     time.Time
		wantErr  bool
	}{
		{"foo.20140504144433.555.log", time.Date(2014, 5, 4, 14, 44, 33, 555000000, time.UTC), false},
		{"foo-20140504144433.555", time.Time{}, true},
		{"20140504144433.555.log", time.Time{}, true},
		{"foo.20140504T144433.555.log", time.Time{}, true},
		{"asdasda.20140504144433.555.log", time.Time{}, true},
		{"foo.log", time.Time{}, true},
		{"foo.xxx.log", time.Time{}, true},
		{"foo.sakdjslkajd", time.Time{}, true},
		{"kjfksjldfjks", time.Time{}, true},
	}

	prefix, ext := splitFilename("asdasdsad/foo.log")

	for _, test := range tests {
		got, err := timeFormFilename(test.filename, prefix, ext)
		assert.Equal(t, test.want, got)
		assert.Equal(t, err != nil, test.wantErr)
	}
}

func TestNewFile(t *testing.T) {
	dir := makeTempDir("TestNewFile", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)
	l := New(LogFile(lf))
	defer l.Close()

	b := []byte("asdfg")
	n, err := l.Write(b)
	require.NoError(t, err)

	assert.Equal(t, len(b), n)
	existsWithContent(lf, b, t)
	fileCount(dir, 1, t)
}

func TestWriteTooLong(t *testing.T) {
	dir := makeTempDir("TestWriteTooLong", t)
	defer os.RemoveAll(dir)

	maxBytes := 10

	lf := logFile(dir)
	l := New(LogFile(lf), MaxBytes(uint64(maxBytes)))
	defer l.Close()

	b := []byte("12345678901")
	n, err := l.Write(b)

	msg := fmt.Sprintf("write length %d exceeds file size limit %d", len(b), l.sizeLimit)
	assert.EqualError(t, err, msg)

	assert.Equal(t, 0, n)

	_, err = os.Stat(lf)
	assert.True(t, os.IsNotExist(err), "File exists, but should not have been created")
}

func TestOpenExisting(t *testing.T) {
	dir := makeTempDir("TestOpenExisting", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)

	data := []byte("foo!")
	err := ioutil.WriteFile(lf, data, 0644)
	require.NoError(t, err)
	existsWithContent(lf, data, t)

	l := New(LogFile(lf))
	defer l.Close()

	b := []byte("12345678901")
	n, err := l.Write(b)

	require.NoError(t, err)
	assert.Equal(t, len(b), n)

	existsWithContent(lf, append(data, b...), t)
	fileCount(dir, 1, t)
}

func TestMakeLogDir(t *testing.T) {
	dir := time.Now().Format("TestMakeLogDir" + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	defer os.RemoveAll(dir)

	lf := logFile(dir)
	l := New(LogFile(lf))
	defer l.Close()

	b := []byte("asdfg")
	n, err := l.Write(b)
	require.NoError(t, err)

	assert.Equal(t, len(b), n)
	existsWithContent(lf, b, t)
	fileCount(dir, 1, t)
}

func TestFilterBackups(t *testing.T) {
	dir := makeTempDir("TestFilterBackups", t)
	defer os.RemoveAll(dir)

	lf := filepath.Join(dir, "foo.log")

	files := []string{
		"foo.20140504144133.555.log",
		"foo.20140504144233.556.log.gz",
		"foo.20140504144333.555.log",
		"foo.20140504144433.556.log.gz",
		"foo.20140504144533.558.log.gz",
		"bar.log",
		"bar",
		"barbar.sdkfjsdfk.log",
		"barbar.log",
		"bar.txt",
		"foo.log",
	}

	// Create test set of files
	for _, f := range files {
		require.NoError(t, ioutil.WriteFile(filepath.Join(dir, f), []byte(f), 0644))
	}

	lst, err := filterBackups(lf)
	require.NoError(t, err)
	assert.Equal(t, 5, len(lst))

	prev := backupInfo{}
	for i, cur := range lst {

		if i == 0 { // Skip first
			prev = cur
			continue
		}

		assert.True(t, cur.timestamp.Before(prev.timestamp), "invalid sorting order")
		prev = cur
	}
}

func TestCollectFilesForSweep(t *testing.T) {
	dir := makeTempDir("TestCollectFilesForSweep", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)
	l := New(LogFile(lf), MaxBytes(10))

	b := []byte("123456789")

	for i := 0; i < 10; i++ {
		n, err := l.Write(b)
		require.NoError(t, err)
		assert.Equal(t, len(b), n)
	}

	fileCount(dir, 10, t)

	forRemove, forCompress, err := l.collectFilesForSweep()

	require.NoError(t, err)
	assert.Equal(t, 0, len(forRemove))
	assert.Equal(t, 0, len(forCompress))

	require.NoError(t, l.Close())

	l = New(LogFile(lf), MaxBytes(10), Compression(true))

	forRemove, forCompress, err = l.collectFilesForSweep()

	require.NoError(t, err)
	assert.Equal(t, 0, len(forRemove))
	assert.Equal(t, 9, len(forCompress))

	require.NoError(t, l.Close())

	l = New(LogFile(lf), MaxBytes(10), Compression(true), MaxBackups(2))

	forRemove, forCompress, err = l.collectFilesForSweep()
	require.NoError(t, err)
	assert.Equal(t, 7, len(forRemove))
	assert.Equal(t, 2, len(forCompress))

	require.NoError(t, l.Close())

	l = New(LogFile(lf), MaxBytes(10), MaxBackups(4))

	forRemove, forCompress, err = l.collectFilesForSweep()
	require.NoError(t, err)
	assert.Equal(t, 5, len(forRemove))
	assert.Equal(t, 0, len(forCompress))

	require.NoError(t, os.Rename(forRemove[0], forRemove[0]+compressSuffix))
	require.NoError(t, l.Close())

	l = New(LogFile(lf), MaxBytes(10), MaxBackups(2), Compression(true))

	forRemove, forCompress, err = l.collectFilesForSweep()
	require.NoError(t, err)
	assert.Equal(t, 7, len(forRemove))
	assert.Equal(t, 2, len(forCompress))

	require.NoError(t, os.Rename(forCompress[0], forCompress[0]+compressSuffix))

	forRemove, forCompress, err = l.collectFilesForSweep()
	require.NoError(t, err)
	assert.Equal(t, 7, len(forRemove))
	assert.Equal(t, 1, len(forCompress))

	require.NoError(t, l.Close())

	l = New(LogFile(lf), MaxBytes(10), MaxAge(1))

	forRemove, forCompress, err = l.collectFilesForSweep()
	require.NoError(t, err)
	assert.Equal(t, 0, len(forRemove), "no need remove")
	assert.Equal(t, 0, len(forCompress), "no need compression")

	diff := time.Duration(int64(24*time.Hour) * int64(2))
	cutoff := time.Now().Add(-1 * diff)

	prefix, suffix := splitFilename(lf)
	oldFileName := filepath.Join(dir, prefix+cutoff.UTC().Format(backupTimeFormat)+suffix)

	require.NoError(t, ioutil.WriteFile(oldFileName, b, fileMode))

	forRemove, forCompress, err = l.collectFilesForSweep()
	require.NoError(t, err)
	assert.Equal(t, 1, len(forRemove))
	assert.Equal(t, 0, len(forCompress))

	require.NoError(t, l.Close())

	l = New(LogFile(lf), MaxBytes(10), Compression(true), MaxAge(1))

	forRemove, forCompress, err = l.collectFilesForSweep()
	require.NoError(t, err)
	assert.Equal(t, 1, len(forRemove))
	assert.Equal(t, 7, len(forCompress))

	require.NoError(t, l.Close())
	l = New(LogFile(lf), MaxBytes(10), Compression(true), MaxBackups(6))

	forRemove, forCompress, err = l.collectFilesForSweep()
	require.NoError(t, err)
	assert.Equal(t, 4, len(forRemove))
	assert.Equal(t, 5, len(forCompress))

	for _, r := range forRemove {
		require.NoError(t, os.Remove(r))
	}
	require.NoError(t, l.Close())
}

func TestCleanup(t *testing.T) {
	dir := makeTempDir("TestCleanup", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)
	l := New(LogFile(lf), MaxBackups(1), MaxBytes(10))
	defer l.Close()

	b := []byte("123456789")

	for i := 0; i < 5; i++ {
		n, err := l.Write(b)
		require.NoError(t, err)
		assert.Equal(t, len(b), n)
	}

	l.wg.Wait()

	fileCount(dir, 2, t)
	assert.True(t, atomic.LoadInt32(&l.sweepings) == 0)
}

func TestCompressing(t *testing.T) {
	dir := makeTempDir("TestCompressing", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)
	l := New(LogFile(lf), MaxBytes(10), MaxBackups(1), Compression(true))

	b := []byte("123456789")

	for i := 0; i < 5; i++ {
		n, err := l.Write(b)
		require.NoError(t, err)
		assert.Equal(t, len(b), n)
	}

	l.wg.Wait()

	require.NoError(t, l.Close())

	assert.True(t, atomic.LoadInt32(&l.sweepings) == 0)
	fileCount(dir, 2, t)
	gzFileCount(dir, 1, t)
}

func TestConcurency(t *testing.T) {
	dir := makeTempDir("TestCompressing", t)
	defer os.RemoveAll(dir)

	lf := logFile(dir)
	l := New(LogFile(lf), MaxBytes(10000), MaxBackups(1), Compression(true))
	defer l.Close()

	b := []byte("123456789")

	count := 5
	wg := sync.WaitGroup{}
	wg.Add(5)

	for i := 0; i < count; i++ {
		go func() {
			for i := 0; i < count; i++ {
				n, err := l.Write(b)
				require.NoError(t, err)
				assert.Equal(t, len(b), n)
			}
			wg.Done()
		}()
	}

	wg.Wait()

	assert.True(t, atomic.LoadInt32(&l.sweepings) == 0)
	fileCount(dir, 1, t)

	info, err := os.Stat(lf)
	require.NoError(t, err)
	assert.Equal(t, int64(count*count*len(b)), info.Size())

	gzFileCount(dir, 0, t)
}

func makeTempDir(name string, t *testing.T) string {
	dir := time.Now().Format(name + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	require.NoError(t, os.Mkdir(dir, 0700))
	return dir
}

func fileCount(dir string, exp int, t testing.TB) {
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)

	// Make sure no other files were created.
	assert.Equal(t, exp, len(files))
}

func gzFileCount(dir string, exp int, t testing.TB) {
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)

	var count int
	for _, f := range files {
		if strings.HasSuffix(f.Name(), compressSuffix) {
			count++
		}
	}

	assert.Equal(t, exp, count)
}

func existsWithContent(path string, content []byte, t testing.TB) {
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), info.Size())

	b, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, b)
}

func logFile(dir string) string {
	return filepath.Join(dir, "foobar.log")
}