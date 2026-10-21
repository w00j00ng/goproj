package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

type config struct {
	addr         string
	maxBodyBytes int64
	logBody      bool
}

func parseConfig(args []string, output io.Writer) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("helloworld", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&cfg.addr, "addr", "127.0.0.1:1323", "HTTP listen address")
	flags.Int64Var(&cfg.maxBodyBytes, "max-body-bytes", 1<<20, "maximum request body bytes (positive)")
	flags.BoolVar(&cfg.logBody, "log-body", false, "log raw bodies and parsed forms, including sensitive values")
	if err := flags.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.maxBodyBytes <= 0 {
		return cfg, errors.New("max-body-bytes must be positive")
	}
	if flags.NArg() != 0 {
		return cfg, errors.New("unexpected positional arguments")
	}
	return cfg, nil
}

func main() {
	cfg, err := parseConfig(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	e := newServer(cfg, log.Default())
	if err := e.Start(cfg.addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func newServer(cfg config, logger *log.Logger) *echo.Echo {
	e := echo.New()
	e.Any("/*", func(c echo.Context) error {
		req := c.Request()
		headers := req.Header.Clone()
		for name := range headers {
			switch strings.ToLower(name) {
			case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key":
				headers[name] = []string{"[REDACTED]"}
			}
		}
		logger.Printf("req.RemoteAddr: %s", req.RemoteAddr)
		logger.Printf("req.Path: %s", req.URL.EscapedPath())
		logger.Printf("req.Method: %s", req.Method)
		logger.Printf("req.Header: %v", headers)

		req.Body = http.MaxBytesReader(c.Response(), req.Body, cfg.maxBodyBytes)
		defer req.Body.Close()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				logger.Print("req.Error: request body too large")
				return c.String(http.StatusRequestEntityTooLarge, "Request body too large")
			}
			logger.Print("req.Error: cannot read request body")
			return c.String(http.StatusBadRequest, "Cannot read request body")
		}
		if cfg.logBody {
			req.Body = http.MaxBytesReader(c.Response(), io.NopCloser(bytes.NewReader(body)), cfg.maxBodyBytes)
			parseErr := req.ParseForm()
			contentType := req.Header.Get("Content-Type")
			if parseErr == nil && contentType != "" {
				contentType, _, parseErr = mime.ParseMediaType(contentType)
				if parseErr == nil && contentType == "multipart/form-data" {
					parseErr = req.ParseMultipartForm(1 << 20)
				}
			}
			if req.MultipartForm != nil {
				defer req.MultipartForm.RemoveAll()
			}
			if parseErr != nil {
				logger.Print("req.Error: cannot parse request form")
				return c.String(http.StatusBadRequest, "Cannot parse request form")
			}
			logger.Printf("req.Body: %s", body)
			logger.Printf("req.Form: %v", req.Form)
			logger.Printf("req.MultipartForm: %v", req.MultipartForm)
		}
		return c.String(http.StatusOK, "OK")
	})
	return e
}
