package rollinglog

// Option func type
type Option func(l *Logger)

// Options turns a list of Option instances into an Option.
func Options(opts ...Option) Option {
	return func(l *Logger) {
		for _, opt := range opts {
			opt(l)
		}
	}
}

// WithLogFile sets output file name with path
func WithLogFile(aFilename string) Option {
	return func(l *Logger) {
		l.filename = aFilename
	}
}

// WithMaxBytes limits log size in bytes. When limit exceeded log will be rotated.
// (0 - never rotate)
func WithMaxBytes(aSize uint64) Option {
	return func(l *Logger) {
		l.sizeLimit = aSize
	}
}

// WithMaxBackups sets the max count of backups to store (0 - no limit)
func WithMaxBackups(aCount int) Option {
	return func(l *Logger) {
		l.backupsCountLimit = aCount
	}
}

// WithMaxAge sets the number of days to store backups (0 - no limit)
func WithMaxAge(aDays int) Option {
	return func(l *Logger) {
		l.backupsDaysLimit = aDays
	}
}

// UseCompression allows to enable compression for backups (disabled by default)
var UseCompression = func(l *Logger) {
	l.compress = true
}

// UseLocaltime allows use local time for timestamps (UTC by default)
var UseLocaltime = func(l *Logger) {
	l.localtime = true
}

// WithErrorHandler allows to set error handler for logger
func WithErrorHandler(eh ErrHandler) Option {
	return func(l *Logger) {
		if eh == nil {
			l.errHandler = defaultErrorHandler
		} else {
			l.errHandler = eh
		}
	}
}
