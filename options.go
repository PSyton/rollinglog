package rollinglog

// Option func type
type Option func(l *Logger)

// LogFile sets output file name with path
func LogFile(aFilename string) Option {
	return func(l *Logger) {
		l.filename = aFilename
	}
}

// MaxBytes limits log size in bytes. When limit exceeded log will be rotated.
// (0 - never rotate)
func MaxBytes(aSize uint64) Option {
	return func(l *Logger) {
		l.sizeLimit = aSize
	}
}

// MaxBackups sets the max count of backups to store (0 - no limit)
func MaxBackups(aCount int) Option {
	return func(l *Logger) {
		l.backupsCountLimit = aCount
	}
}

// MaxAge sets the number of days to store backups (0 - no limit)
func MaxAge(aDays int) Option {
	return func(l *Logger) {
		l.backupsDaysLimit = aDays
	}
}

// Compression allows to enable compression for backups (disabled by default)
func Compression(aCompression bool) Option {
	return func(l *Logger) {
		l.compress = aCompression
	}
}

// Localtime allows use local time for timestamps (UTC by default)
func Localtime(aLocaltime bool) Option {
	return func(l *Logger) {
		l.localtime = aLocaltime
	}
}

// ErrorHandler allows to set error handler for logger
func ErrorHandler(eh ErrHandler) Option {
	return func(l *Logger) {
		if eh == nil {
			l.errHandler = defaultErrorHandler
		} else {
			l.errHandler = eh
		}
	}
}
