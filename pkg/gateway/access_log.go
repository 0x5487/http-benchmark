package gateway

import (
	"bufio"
	"context"
	"http-benchmark/pkg/domain"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/valyala/bytebufferpool"
)

type LoggerTracer struct {
	opts      domain.AccessLogOptions
	matchVars []string
	logChan   chan string
	logFile   *os.File
	writer    *bufio.Writer
}

func NewLoggerTracer(opts domain.AccessLogOptions) (*LoggerTracer, error) {

	if opts.TimeFormat == "" {
		opts.TimeFormat = time.RFC3339
	}

	words := strings.Fields(opts.Template)
	opts.Template = strings.Join(words, " ") + "\n"

	logFile, err := os.OpenFile(opts.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	if opts.BufferSize <= 0 {
		opts.BufferSize = 64 * 1024
	}

	writer := bufio.NewWriterSize(logFile, opts.BufferSize)

	if opts.Flush.Seconds() <= 0 {
		opts.Flush = 1 * time.Second
	}

	tracer := &LoggerTracer{
		opts:      opts,
		logChan:   make(chan string, 1000000),
		matchVars: parseVariables(opts.Template),
		logFile:   logFile,
		writer:    writer,
	}

	go func(t *LoggerTracer) {
		flushTimer := time.NewTimer(opts.Flush)

		for {
			flushTimer.Reset(opts.Flush)

			select {
			case entry, ok := <-t.logChan:
				if !ok {
					// Channel closed, flush remaining data
					writer.Flush()
					return
				}
				_, _ = writer.WriteString(entry)
			case <-flushTimer.C:
				_ = writer.Flush()
				_ = t.logFile.Sync()
			}
		}
	}(tracer)

	return tracer, nil
}

func (t *LoggerTracer) Start(ctx context.Context, c *app.RequestContext) context.Context {
	time := time.Now().UTC()
	c.Set(domain.TIME, time)
	return ctx
}

func (t *LoggerTracer) Finish(ctx context.Context, c *app.RequestContext) {
	replacer := t.buildReplacer(c)
	result := replacer.Replace(t.opts.Template)
	t.logChan <- result
}

func (t *LoggerTracer) Shutdown() {
	close(t.logChan)
	t.writer.Flush()
	_ = t.logFile.Sync()
	t.logFile.Close()
}

func (t *LoggerTracer) buildReplacer(c *app.RequestContext) *strings.Replacer {
	replacements := make([]string, 0, len(t.matchVars)*2)

	for _, matchVal := range t.matchVars {
		switch matchVal {
		case domain.TIME:
			startTime := c.GetTime(domain.TIME)
			replacements = append(replacements, domain.TIME, startTime.Format(t.opts.TimeFormat))
		case domain.REMOTE_ADDR:
			var ip string
			switch addr := c.RemoteAddr().(type) {
			case *net.UDPAddr:
				ip = addr.IP.String()
			case *net.TCPAddr:
				ip = addr.IP.String()
			}
			replacements = append(replacements, domain.REMOTE_ADDR, ip)
		case domain.REQUEST_METHOD:
			replacements = append(replacements, domain.REQUEST_METHOD, b2s(c.Request.Method()))
		case domain.UPSTREAM_METHOD:
			replacements = append(replacements, domain.REQUEST_METHOD, b2s(c.Request.Method()))
		case domain.REQUEST_URI:
			buf := bytebufferpool.Get()
			defer bytebufferpool.Put(buf)

			val, found := c.Get(domain.REQUEST_PATH)
			if found {
				path, ok := val.(string)
				if ok {
					_, _ = buf.WriteString(path)
					if len(c.Request.QueryString()) > 0 {
						_, _ = buf.Write(questionByte)
						_, _ = buf.Write(c.Request.QueryString())
					}

					replacements = append(replacements, domain.REQUEST_URI, buf.String())
				}
				continue
			}

			_, _ = buf.Write(c.Request.Path())
			if len(c.Request.QueryString()) > 0 {
				_, _ = buf.Write(questionByte)
				_, _ = buf.Write(c.Request.QueryString())
			}
			replacements = append(replacements, domain.REQUEST_URI, buf.String())

		case domain.REQUEST_PATH:
			val, found := c.Get(domain.REQUEST_PATH)
			if found {
				b, ok := val.([]byte)
				if ok {
					replacements = append(replacements, domain.REQUEST_PATH, b2s(b))
					continue
				} else {
					replacements = append(replacements, domain.REQUEST_PATH, "")
				}
				continue
			}
			replacements = append(replacements, domain.REQUEST_PATH, b2s(c.Request.Path()))
		case domain.REQUEST_PROTOCOL:
			replacements = append(replacements, domain.REQUEST_PROTOCOL, c.Request.Header.GetProtocol())
		case domain.REQUEST_BODY:
			body := escape(b2s(c.Request.Body()), t.opts.Escape)
			replacements = append(replacements, domain.REQUEST_BODY, body)
		case domain.STATUS:
		case domain.UPSTREAM_STATUS:
			replacements = append(replacements, domain.STATUS, strconv.Itoa(c.Response.StatusCode()))
		case domain.UPSTREAM_PROTOCOL:
			replacements = append(replacements, domain.UPSTREAM_PROTOCOL, c.Request.Header.GetProtocol())
		case domain.UPSTREAM_URI:
			buf := bytebufferpool.Get()
			defer bytebufferpool.Put(buf)

			_, _ = buf.Write(c.Request.Path())

			if len(c.Request.QueryString()) > 0 {
				_, _ = buf.Write(questionByte)
				_, _ = buf.Write(c.Request.QueryString())
			}

			replacements = append(replacements, domain.UPSTREAM_URI, buf.String())
		case domain.UPSTREAM_PATH:
			replacements = append(replacements, domain.UPSTREAM_PATH, b2s(c.Request.Path()))
		case domain.UPSTREAM_ADDR:
			addr := c.GetString(domain.UPSTREAM_ADDR)
			replacements = append(replacements, domain.UPSTREAM_ADDR, addr)
		case domain.UPSTREAM_RESPONSE_TIME:
			replacements = append(replacements, domain.UPSTREAM_RESPONSE_TIME, c.GetString(domain.UPSTREAM_RESPONSE_TIME))
		case domain.DURATION:
			dur := time.Since(c.GetTime(domain.TIME)).Microseconds()
			duration := strconv.FormatFloat(float64(dur)/1e6, 'f', -1, 64)
			replacements = append(replacements, domain.DURATION, duration)
		default:

			if strings.HasPrefix(matchVal, "$upstream_header_") {
				headerVal := matchVal[len("$upstream_header_"):]
				headerVal = c.Response.Header.Get(headerVal)
				headerVal = escape(headerVal, t.opts.Escape)
				replacements = append(replacements, matchVal, headerVal)
				continue
			}

			if strings.HasPrefix(matchVal, "$header_") {
				headerVal := matchVal[len("$header_"):]

				if headerVal == "X-Forwarded-For" {
					ip := c.GetString("X-Forwarded-For")
					replacements = append(replacements, matchVal, ip)
					continue
				}

				headerVal = c.Request.Header.Get(headerVal)
				headerVal = escape(headerVal, t.opts.Escape)
				replacements = append(replacements, matchVal, headerVal)
				continue
			}

			replacements = append(replacements, matchVal, matchVal)
		}
	}

	replacer := strings.NewReplacer(replacements...)

	return replacer
}

func escape(s string, escapeType domain.EscapeType) string {
	if len(s) == 0 {
		return s
	}

	switch escapeType {
	case domain.DefaultEscape:
		s = escapeString(s)
	case domain.JSONEscape:
		s = escapeJSON(s)
	case domain.NoneEscape:
		return s
	}

	return s
}

// escapeString function to escape special characters
func escapeString(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c == '"' || c == '\\' || c < 32 || c > 126 {
			b.WriteString(`\x`)
			b.WriteString(strconv.FormatUint(uint64(c), 16))
			i++
		} else {
			r, size := utf8.DecodeRuneInString(s[i:])
			b.WriteRune(r)
			i += size
		}
	}
	return b.String()
}

// escapeJSON function to escape characters for JSON strings
func escapeJSON(s string) string {
	b, err := sonic.Marshal(s)
	if err != nil {
		return s
	}

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	_, _ = buf.Write(b)
	return buf.String()
}
