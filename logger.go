package logger

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Log  *zap.Logger
	once sync.Once
)

// LogConfig holds logging configuration
type LogConfig struct {
	// Directory for log files
	LogDir string
	// Filename for the log file
	Filename string
	// MaxSize is the maximum size in megabytes before rotation
	MaxSize int
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int
	// MaxAge is the maximum number of days to retain old log files
	MaxAge int
	// Compress determines if rotated files should be compressed
	Compress bool
	// Level is the minimum log level
	Level zapcore.Level
	// EnableConsole enables console output
	EnableConsole bool
	// ConsoleLevel is the minimum level for console output
	ConsoleLevel zapcore.Level
}

// DefaultLogConfig returns sensible defaults for logging
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		LogDir:        "logs",
		Filename:      "app.log",
		MaxSize:       100,  // 100 MB
		MaxBackups:    3,    // Keep 3 old files
		MaxAge:        28,   // 28 days
		Compress:      true, // Compress old files
		Level:         zapcore.InfoLevel,
		EnableConsole: true,
		ConsoleLevel:  zapcore.DebugLevel,
	}
}

func InitLogger() {
	InitLoggerWithConfig(DefaultLogConfig())
}

func InitLoggerWithConfig(cfg *LogConfig) {
	once.Do(func() {
		if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
			panic("failed to create log directory: " + err.Error())
		}

		env := os.Getenv("ENV")
		isProduction := env == "production"

		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

		logPath := filepath.Join(cfg.LogDir, cfg.Filename)

		fileWriter := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}

		fileLevel := cfg.Level
		if isProduction {
			fileLevel = zapcore.InfoLevel
		}

		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(fileWriter),
			fileLevel,
		)

		cores := []zapcore.Core{fileCore}

		if cfg.EnableConsole {
			consoleCfg := encoderConfig
			consoleCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

			var enc zapcore.Encoder
			if isProduction {
				enc = zapcore.NewJSONEncoder(consoleCfg)
			} else {
				enc = zapcore.NewConsoleEncoder(consoleCfg)
			}

			consoleCore := zapcore.NewCore(
				enc,
				zapcore.AddSync(os.Stdout),
				cfg.ConsoleLevel,
			)

			cores = append(cores, consoleCore)
		}

		Log = zap.New(
			zapcore.NewTee(cores...),
			zap.AddCaller(),
			zap.AddCallerSkip(1),
			zap.AddStacktrace(zapcore.ErrorLevel),
		)
	})
}

// WithContext returns a logger with additional context fields
func WithContext(fields ...zap.Field) *zap.Logger {
	InitLogger()
	return Log.With(fields...)
}

// WithRequestID returns a logger with request ID field
func WithRequestID(requestID string) *zap.Logger {
	return WithContext(zap.String("request_id", requestID))
}

// WithUserID returns a logger with user ID field
func WithUserID(userID string) *zap.Logger {
	return WithContext(zap.String("user_id", userID))
}

// NewRequestLogger creates a logger for a specific request
func NewRequestLogger(requestID, userID, method, path, clientIP string) *zap.Logger {
	return WithContext(
		zap.String("request_id", requestID),
		zap.String("user_id", userID),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("client_ip", clientIP),
	)
}

func Info(msg string, fields ...zap.Field) {
	InitLogger()
	Log.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	InitLogger()
	Log.Error(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	InitLogger()
	Log.Debug(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	InitLogger()
	Log.Warn(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	InitLogger()
	Log.Fatal(msg, fields...)
}

func GetLogger() *zap.Logger {
	InitLogger()
	return Log
}

// Sync flushes any buffered log entries
func Sync() error {
	if Log == nil {
		return nil
	}

	err := Log.Sync()

	if err == nil {
		return nil
	}

	if errors.Is(err, syscall.EINVAL) {
		return nil
	}

	return err
}

// Close syncs and closes the logger
func Close() error {
	return Sync()
}

// SetOutput sets a custom writer for the logger (useful for testing)
func SetOutput(w io.Writer) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(w),
		zapcore.DebugLevel,
	)

	Log = zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	)
}
