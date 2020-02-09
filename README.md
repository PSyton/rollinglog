# rollinglog [![Build Status](https://github.com/PSyton/rollinglog/workflows/test/badge.svg)](https://github.com/PSyton/rollinglog/actions) [![Go Report Card](https://goreportcard.com/badge/github.com/PSyton/rollinglog)](https://goreportcard.com/report/github.com/PSyton/rollinglog) [![Coverage Status](https://coveralls.io/repos/github/PSyton/rollinglog/badge.svg?branch=master)](https://coveralls.io/github/PSyton/rollinglog?branch=master)

Package provides a simple rolling logger

Inspired by lumberjack (<https://github.com/natefinch/lumberjack>)

Rollinglog is intended to be one part of a logging infrastructure.
It is not an all-in-one solution, but instead is a pluggable
component at the bottom of the logging stack that simply controls the files
to which logs are written.

Rollinglog plays well with any logging package that can write to an
io.Writer, including the standard library's log package.

## Install

go get github.com/PSyton/rollinglog

## Usage

One can use rollinglog with the standard library's log package, just pass it into the SetOutput function when your application starts.

```go
import "github.com/PSyton/rollinglog"

// Create logger writes log into file.log in current folder and limits maximum size by 10Mb
// No limits for backups and no compresion enabled
logger := rollinglog.New(rollinglog.LogFile("file.log"), rollinglog.MaxBytes(10 * 1024 * 1024))

log.SetOutput(logger)

...
// Close the logger
l.Close()

```

## Details

Logger is an `io.WriteCloser` that writes to the specified file.

Logger opens or creates the logfile on first Write. If the file exists and
is less than *MaxBytes* bytes, rollinglog will open and append to that file. If the file exists and its size is larger *MaxBytes*, the file is renamed by putting the current time in a timestamp in the name immediately before the file's extension (or the end of the filename if there's no extension). A new log file is then created using original filename.

Whenever a write would cause the current log file exceed *MaxBytes* bytes,
the current file is closed, renamed, and a new log file created with the
original name. Thus, the filename you give Logger is always the *"current"* log file.

Backups use the log file name given to Logger, in the form `name.timestamp.ext` where name is the filename without the extension, timestamp is the time at which the log was rotated formatted with the time.Time format of TimeFormat and the extension is the original extension. For example, if your *LogFile* is `/var/log/foo/server.log`, a backup created
at 6:30pm on Nov 11 2016 would use the filename `/var/log/foo/server.20161104183000.000.log`

### Cleaning Up Old Log Files

Whenever a new logfile gets created, old log files may be deleted. The most recent files according to the encoded timestamp will be retained, up to a number equal to *MaxBackups* (or all of them if *MaxBackups* is 0). Any files with an encoded timestamp older than MaxAge days are deleted, regardless of *MaxBackups*. Note that the time encoded in the timestamp is the rotation time, which may differ from the last time that file was written to.

If *MaxBackups* and *MaxAge* are both 0, no old log files will be deleted.

### Options

`rollinglog.New` accepts functional options:

* `rollinglog.LogFile(aFilnename string)` - sets log file name with path. By default logger use file name `os.Args[0]-rollinglog.log` and place it in `os.TempDir()`
* `rollinglog.MaxBytes(aSize uint64)` - limits log size in bytes. When limit exceeded log will be rotated. (Defailt: 0 - never rotate)
* `rollinglog.MaxBackups(aCount int)` - sets the max count of backups to store (Default: 0 - no limit)
* `rollinglog.MaxAge(aDays int)` - sets the number of days to store backups (Default: 0 - no limit)
* `rollinglog.Compression(aCompression bool)` - allows to enable compression for backups (disabled by default)
* `rollinglog.Localtime(aLocaltime bool)` - allows use local time for timestamps instead default UTC
* `rollinglog.ErrorHandler(eh ErrHandler)` - allows to set error handler for logger.
