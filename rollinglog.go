package rollinglog

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

const (
	backupTimeFormat string = "20060102150405.000"
	compressSuffix   string = ".gz"
	fileMode                = 0644
)

// ErrHandler function called on error in logging
type ErrHandler func(error)

// default handler do nothing
var defaultErrorHandler ErrHandler = func(err error) {
}

// ensure we always implement io.WriteCloser
var _ io.WriteCloser = (*Logger)(nil)

// Logger provide functional for wtore logs
type Logger struct {
	filename          string
	sizeLimit         uint64
	backupsDaysLimit  int
	backupsCountLimit int
	compress          bool
	localtime         bool
	errHandler        ErrHandler

	size     uint64
	file     *os.File
	lock     sync.Mutex
	wg       sync.WaitGroup
	shutdown int32

	sweepings int32
}

// New create logger for log writed to aFilename
func New(options ...Option) *Logger {
	name := filepath.Base(os.Args[0]) + "-rollinglog.log"

	l := &Logger{
		filename:   filepath.Join(os.TempDir(), name),
		errHandler: defaultErrorHandler,
	}

	for _, o := range options {
		o(l)
	}

	return l
}

func (l *Logger) runSweeping() {
	// No need any post rotate actions
	if l.backupsDaysLimit == 0 && l.backupsCountLimit == 0 && !l.compress {
		return
	}

	if atomic.LoadInt32(&l.sweepings) == 0 {
		l.wg.Add(1)
		go l.sweep()
	}
}

func (l *Logger) collectFilesForSweep() (forRemove, forCompress []string, err error) {
	// Get all backups for current log file
	backups, err := filterBackups(l.filename)

	if err != nil {
		return nil, nil, err
	}

	dir := filepath.Dir(l.filename)

	// Doesn't matter compressed backups on not, because
	// compression process remove non compressed file

	// Take old files first
	if l.backupsDaysLimit > 0 {
		diff := time.Duration(int64(24*time.Hour) * int64(l.backupsDaysLimit))
		cutoff := time.Now().Add(-1 * diff)

		// backups ordered by timestamp
		for len(backups) > 0 {
			b := backups[len(backups)-1]
			if b.timestamp.Before(cutoff) {
				forRemove = append(forRemove, filepath.Join(dir, b.name))
				backups = backups[:len(backups)-1]
			} else {
				break
			}
		}
	}

	// Take files under limit
	if l.backupsCountLimit > 0 {
		for l.backupsCountLimit < len(backups) {
			forRemove = append(forRemove, filepath.Join(dir, backups[len(backups)-1].name))
			backups = backups[:len(backups)-1]
		}
	}

	// Check rest for compress
	if l.compress {
		for _, b := range backups {
			if !strings.HasSuffix(b.name, compressSuffix) {
				forCompress = append(forCompress, filepath.Join(dir, b.name))
			}
		}
	}

	return
}

func (l *Logger) needShutdown() bool {
	return atomic.LoadInt32(&l.shutdown) == 1
}

func (l *Logger) sweep() {
	atomic.StoreInt32(&l.sweepings, 1)
	defer func() {
		atomic.StoreInt32(&l.sweepings, 0)
		l.wg.Done()
	}()

	// Trying while has to do something
	for {
		if l.needShutdown() {
			break
		}
		forRemove, forCompress, err := l.collectFilesForSweep()

		if len(forRemove) == 0 && len(forCompress) == 0 {
			// Nothong todo
			if err != nil {
				l.errHandler(err)
			}
			return
		}

		for _, r := range forRemove {
			if err := os.Remove(r); err != nil {
				l.errHandler(err)
			}
		}

		for _, f := range forCompress {
			if l.needShutdown() {
				break
			}

			if err := newCompressor(f).Compress(); err != nil {
				l.errHandler(err)
				// Stop when has errors. We'll try another time
				return
			}
		}
	}
}

func (l *Logger) rotate() error {
	dir, fname := filepath.Split(l.filename)

	prefix, suffix := splitFilename(fname)

	t := time.Now()
	if !l.localtime {
		t = t.UTC()
	}

	ts := t.Format(backupTimeFormat)
	backupFile := filepath.Join(dir, fmt.Sprintf("%s%s%s", prefix, ts, suffix))
	if err := os.Rename(l.filename, backupFile); err != nil {
		return err
	}

	l.runSweeping()
	return nil
}

func (l *Logger) create() (*os.File, uint64, error) {
	dir := filepath.Dir(l.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, 0, errors.Wrapf(err, "can't make directories for %s", dir)
	}

	f, err := os.OpenFile(l.filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return nil, 0, errors.Wrapf(err, "can't create file %s", l.filename)
	}

	return f, 0, nil
}

func (l *Logger) openOrCreate(aNeedWrite uint64) (*os.File, uint64, error) {
	info, err := os.Stat(l.filename)
	if os.IsNotExist(err) {
		return l.create()
	}
	if err != nil {
		return nil, 0, errors.Wrapf(err, "can't stat %s", l.filename)
	}

	curSize := uint64(info.Size())

	if sizeExceeded(aNeedWrite+curSize, l.sizeLimit) {
		if err = l.rotate(); err != nil {
			return nil, 0, errors.Wrapf(err, "can't rotate %s", l.filename)
		}
		return l.create()
	}

	file, err := os.OpenFile(l.filename, os.O_APPEND|os.O_WRONLY, fileMode)
	if err != nil {
		return nil, 0, errors.Wrapf(err, "can't open %s", l.filename)
	}

	return file, curSize, nil
}

func (l *Logger) close() (err error) {
	f := l.file
	l.size = 0
	l.file = nil

	errs := new(multierror.Error)

	if f != nil {
		errs = multierror.Append(errs, f.Sync())
		errs = multierror.Append(errs, f.Close())
	}

	return errs.ErrorOrNil()
}

// Write implements io.Writer interface
func (l *Logger) Write(p []byte) (n int, err error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	writeLen := uint64(len(p))

	if sizeExceeded(writeLen, l.sizeLimit) {
		return 0, errors.Errorf("write length %d exceeds file size limit %d", writeLen, l.sizeLimit)
	}

	if l.file == nil {
		if l.file, l.size, err = l.openOrCreate(writeLen); err != nil {
			return 0, errors.Wrap(err, "write failed")
		}
	}

	if sizeExceeded(l.size+writeLen, l.sizeLimit) {
		if err = l.close(); err != nil {
			return 0, errors.Wrapf(err, "can't close for rotate on write %s", l.filename)
		}
		if err = l.rotate(); err != nil {
			return 0, errors.Wrapf(err, "can't rotate on write %s", l.filename)
		}
		if l.file, l.size, err = l.create(); err != nil {
			return 0, errors.Wrapf(err, "write failed")
		}
	}

	n, err = l.file.Write(p)
	l.size += uint64(n)

	return n, err
}

// Close implements io.Closer interface
func (l *Logger) Close() error {
	l.lock.Lock()
	defer l.lock.Unlock()

	if atomic.LoadInt32(&l.sweepings) == 1 {
		atomic.StoreInt32(&l.shutdown, 1)
		l.wg.Wait()
		atomic.StoreInt32(&l.shutdown, 0)
	}

	return l.close()
}

// backupInfo is a convenience struct to return the filename and its embedded
// timestamp.
type backupInfo struct {
	name      string
	timestamp time.Time
}

// byTimestamp sorts by newest time formatted in the name.
type byTimestamp []backupInfo

func (b byTimestamp) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

func (b byTimestamp) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byTimestamp) Len() int {
	return len(b)
}

// splitFilename get prefix and suffix for log file (timestamp added between)
func splitFilename(aFileName string) (prefix string, suffix string) {
	_, fname := filepath.Split(aFileName)
	suffix = filepath.Ext(fname)

	prefix = fname[:len(fname)-len(suffix)] + "."
	return
}

// Filter list of files from dir of aBaseFile
// Result sorted by timestamp.
func filterBackups(aLogFilename string) ([]backupInfo, error) {
	files, err := ioutil.ReadDir(filepath.Dir(aLogFilename))
	if err != nil {
		return nil, errors.Wrap(err, "can't read log file directory: %s")
	}

	result := []backupInfo{}

	prefix, suffix := splitFilename(aLogFilename)
	cSiffix := suffix + compressSuffix

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if ts, err := timeFormFilename(f.Name(), prefix, suffix); err == nil {
			result = append(result, backupInfo{f.Name(), ts})
		} else if ts, err := timeFormFilename(f.Name(), prefix, cSiffix); err == nil {
			result = append(result, backupInfo{f.Name(), ts})
		}
	}

	sort.Sort(byTimestamp(result))
	return result, nil
}

func sizeExceeded(aSize, aLimitSize uint64) bool {
	if aLimitSize == 0 {
		return false
	}

	return aSize > aLimitSize
}

func timeFormFilename(aFilename, aPrefix, aSuffix string) (time.Time, error) {
	if !strings.HasPrefix(aFilename, aPrefix) {
		return time.Time{}, errors.New("mismatch prefix")
	}
	if !strings.HasSuffix(aFilename, aSuffix) {
		return time.Time{}, errors.New("mismatch prefix")
	}

	if len(backupTimeFormat) == len(aFilename)-len(aSuffix)-len(aPrefix) {
		ts := aFilename[len(aPrefix) : len(aFilename)-len(aSuffix)]
		return time.Parse(backupTimeFormat, ts)
	}

	return time.Time{}, errors.New("no time field")
}
