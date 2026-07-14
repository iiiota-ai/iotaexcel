// Package logging 提供工具内部使用的轻量结构化日志。
//
// 项目需要兼容 Go 1.19，因此没有使用 Go 1.21 才加入的 log/slog。
// 该实现支持 text/json 两种格式、debug/info/warn/error 四个级别，以及可选文件输出。
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config 是 logger 初始化参数。
// File 为空时日志写到 stderr；非空时写到指定文件并由调用方负责执行返回的 close 函数。
type Config struct {
	Level  string
	Format string
	File   string
}

// Level 表示日志级别，数值越大级别越高。
// log 方法通过比较 level 来过滤低优先级日志。
type Level int

const (
	// LevelDebug 输出最详细的调试信息。
	LevelDebug Level = iota

	// LevelInfo 输出普通进度和汇总信息，是默认级别。
	LevelInfo

	// LevelWarn 输出可继续执行但需要关注的问题。
	LevelWarn

	// LevelError 输出导致当前文件或命令失败的错误。
	LevelError
)

// Logger 是线程安全的结构化日志写入器。
// mu 用于避免并发写入时多行日志交错；当前 CLI 主要是串行流程，但保留该保护成本很低。
type Logger struct {
	out    io.Writer
	level  Level
	format string
	mu     sync.Mutex
}

// New 创建 Logger，并返回一个关闭资源的函数。
// 如果配置了日志文件，会自动创建父目录并以 append 模式打开文件。
func New(cfg Config) (*Logger, func() error, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, nil, err
	}

	var out io.Writer = os.Stderr
	var closeFn func() error = func() error { return nil }
	if cfg.File != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0o755); err != nil {
			return nil, nil, err
		}
		f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, err
		}
		out = f
		closeFn = f.Close
	}

	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	if format == "" {
		format = "text"
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "text":
		return &Logger{out: out, level: level, format: format}, closeFn, nil
	case "json":
		return &Logger{out: out, level: level, format: format}, closeFn, nil
	default:
		return nil, nil, fmt.Errorf("unsupported log format %q", cfg.Format)
	}
}

// Debug 写入 debug 级别日志。
// args 使用 key/value 交替传入，非字符串 key 会被忽略。
func (l *Logger) Debug(msg string, args ...any) {
	l.log(LevelDebug, "DEBUG", msg, args...)
}

// Info 写入 info 级别日志。
// 用于命令开始、汇总统计等正常流程信息。
func (l *Logger) Info(msg string, args ...any) {
	l.log(LevelInfo, "INFO", msg, args...)
}

// Warn 写入 warn 级别日志。
// 用于无输入文件、空行跳过等不会让命令失败但值得关注的情况。
func (l *Logger) Warn(msg string, args ...any) {
	l.log(LevelWarn, "WARN", msg, args...)
}

// Error 写入 error 级别日志。
// 用于文件读取、schema 校验、引用校验或输出失败等错误。
func (l *Logger) Error(msg string, args ...any) {
	l.log(LevelError, "ERROR", msg, args...)
}

// log 执行实际日志格式化和写入。
// JSON 模式输出单行 JSON；text 模式输出 key=value 风格，便于终端查看和简单脚本解析。
func (l *Logger) log(level Level, levelName, msg string, args ...any) {
	if level < l.level {
		return
	}
	fields := map[string]any{
		"time":  time.Now().Format(time.RFC3339),
		"level": levelName,
		"msg":   msg,
	}
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			continue
		}
		fields[key] = args[i+1]
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.format == "json" {
		data, err := json.Marshal(fields)
		if err == nil {
			fmt.Fprintln(l.out, string(data))
			return
		}
	}
	fmt.Fprintf(l.out, "time=%s level=%s msg=%q", fields["time"], levelName, msg)
	for k, v := range fields {
		if k == "time" || k == "level" || k == "msg" {
			continue
		}
		fmt.Fprintf(l.out, " %s=%q", k, fmt.Sprint(v))
	}
	fmt.Fprintln(l.out)
}

// parseLevel 解析命令行传入的日志级别。
// warning 作为 warn 的别名保留，其他未知值会返回错误避免静默降级。
func parseLevel(raw string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return LevelInfo, nil
	case "debug":
		return LevelDebug, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unsupported log level %q", raw)
	}
}
