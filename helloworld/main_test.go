package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func TestCLIInvalidBodyLimit(t *testing.T) {
	for _, value := range []string{"0", "-1"} {
		t.Run(value, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCLIProcess$", "--", "--max-body-bytes="+value)
			cmd.Env = append(os.Environ(), "HELLOWORLD_TEST_PROCESS=1")
			output, err := cmd.CombinedOutput()
			if err == nil || ctx.Err() != nil || !strings.Contains(string(output), "max-body-bytes must be positive") {
				t.Fatalf("invalid limit must exit with validation error; err=%v, output=%s", err, output)
			}
		})
	}
}

func TestCLIProcess(t *testing.T) {
	if os.Getenv("HELLOWORLD_TEST_PROCESS") != "1" {
		return
	}
	os.Args = append([]string{"helloworld"}, os.Args[3:]...)
	main()
	os.Exit(0)
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      config
		wantError bool
	}{
		{name: "defaults", want: config{addr: "127.0.0.1:1323", maxBodyBytes: 1 << 20}},
		{name: "overrides", args: []string{"--addr=0.0.0.0:8080", "--max-body-bytes=7", "--log-body"}, want: config{addr: "0.0.0.0:8080", maxBodyBytes: 7, logBody: true}},
		{name: "zero", args: []string{"--max-body-bytes=0"}, wantError: true},
		{name: "negative", args: []string{"--max-body-bytes=-1"}, wantError: true},
		{name: "noninteger", args: []string{"--max-body-bytes=abc"}, wantError: true},
		{name: "unknown", args: []string{"--unknown"}, wantError: true},
		{name: "positional", args: []string{"extra"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConfig(tt.args, io.Discard)
			if (err != nil) != tt.wantError {
				t.Fatalf("parseConfig error = %v, want error %v", err, tt.wantError)
			}
			if !tt.wantError && got != tt.want {
				t.Fatalf("parseConfig = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRequestBodyLimit(t *testing.T) {
	tests := []struct {
		name          string
		body          io.Reader
		contentLength int64
		limit         int64
		wantStatus    int
	}{
		{"empty", strings.NewReader(""), 0, 4, http.StatusOK},
		{"boundary", strings.NewReader("1234"), 4, 4, http.StatusOK},
		{"exceeds", strings.NewReader("12345"), 5, 4, http.StatusRequestEntityTooLarge},
		{"unknown boundary", strings.NewReader("1234"), -1, 4, http.StatusOK},
		{"unknown exceeds", strings.NewReader("12345"), -1, 4, http.StatusRequestEntityTooLarge},
		{"read error", io.MultiReader(strings.NewReader("PARTIAL_SECRET"), iotest.ErrReader(errors.New("READER_SECRET"))), -1, 64, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			server := newServer(config{maxBodyBytes: tt.limit, logBody: true}, log.New(&logs, "", 0))
			req := httptest.NewRequest(http.MethodPost, "/inspect", tt.body)
			req.ContentLength = tt.contentLength
			response := httptest.NewRecorder()
			server.ServeHTTP(response, req)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; response=%s", response.Code, tt.wantStatus, response.Body)
			}
			if tt.wantStatus == http.StatusOK && response.Body.String() != "OK" {
				t.Fatalf("response = %q, want OK", response.Body.String())
			}
			for _, secret := range []string{"PARTIAL_SECRET", "READER_SECRET"} {
				if strings.Contains(logs.String()+response.Body.String(), secret) {
					t.Errorf("error handling exposed %s", secret)
				}
			}
			if tt.wantStatus != http.StatusOK && strings.Contains(logs.String(), "req.Body:") {
				t.Errorf("rejected request logged its body: %s", logs.String())
			}
		})
	}
}

func TestRequestLogging(t *testing.T) {
	for _, logBody := range []bool{false, true} {
		name := "default"
		if logBody {
			name = "opt in"
		}
		t.Run(name, func(t *testing.T) {
			var logs bytes.Buffer
			server := newServer(config{maxBodyBytes: 1024, logBody: logBody}, log.New(&logs, "", 0))
			req := httptest.NewRequest(http.MethodPost, "/inspect?query=QUERY_SECRET", strings.NewReader("form=FORM_SECRET"))
			req.Header = http.Header{
				"Content-Type":        {"application/x-www-form-urlencoded"},
				"aUtHoRiZaTiOn":       {"AUTH_SECRET"},
				"proxy-authorization": {"PROXY_SECRET"},
				"COOKIE":              {"COOKIE_SECRET"},
				"Set-Cookie":          {"SET_COOKIE_SECRET"},
				"x-api-key":           {"API_SECRET"},
				"X-Trace":             {"trace-id"},
			}
			original := req.Header.Clone()
			response := httptest.NewRecorder()
			server.ServeHTTP(response, req)
			if response.Code != http.StatusOK || response.Body.String() != "OK" {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
			if !reflect.DeepEqual(req.Header, original) {
				t.Error("request headers were changed")
			}
			output := logs.String()
			for _, want := range []string{"POST", "/inspect", "X-Trace", "trace-id", "[REDACTED]"} {
				if !strings.Contains(output, want) {
					t.Errorf("log lacks %q: %s", want, output)
				}
			}
			for _, secret := range []string{"AUTH_SECRET", "PROXY_SECRET", "COOKIE_SECRET", "SET_COOKIE_SECRET", "API_SECRET"} {
				if strings.Contains(output, secret) {
					t.Errorf("header secret logged: %s", secret)
				}
			}
			if logBody {
				for _, want := range []string{"req.Body: form=FORM_SECRET", "form:[FORM_SECRET]", "query:[QUERY_SECRET]"} {
					if !strings.Contains(output, want) {
						t.Errorf("opt-in log lacks %q: %s", want, output)
					}
				}
			} else {
				for _, secret := range []string{"QUERY_SECRET", "FORM_SECRET", "req.Body:", "req.Form:", "req.MultipartForm:"} {
					if strings.Contains(output, secret) {
						t.Errorf("default log exposed %q: %s", secret, output)
					}
				}
			}
		})
	}
}

func TestMalformedForm(t *testing.T) {
	tests := []struct{ name, method, contentType, target, body string }{
		{"urlencoded", http.MethodPost, "application/x-www-form-urlencoded", "/", "field=FORM_SECRET%zz"},
		{"multipart", http.MethodPost, "multipart/form-data; boundary=secret", "/", "--secret\r\nFORM_SECRET"},
		{"query", http.MethodPost, "application/x-www-form-urlencoded", "/?query=QUERY_SECRET%zz", "field=FORM_SECRET"},
		{"content type", http.MethodPost, "application/x-www-form-urlencoded; invalid", "/", "field=FORM_SECRET"},
		{"GET content type", http.MethodGet, "application/x-www-form-urlencoded; invalid", "/", "field=FORM_SECRET%zz"},
		{"DELETE content type", http.MethodDelete, "application/x-www-form-urlencoded; invalid", "/", "field=FORM_SECRET%zz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			server := newServer(config{maxBodyBytes: 1024, logBody: true}, log.New(&logs, "", 0))
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, req)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
			for _, secret := range []string{"FORM_SECRET", "QUERY_SECRET", "invalid URL escape", "unexpected EOF", "mime: invalid media parameter"} {
				if strings.Contains(logs.String()+response.Body.String(), secret) {
					t.Errorf("parse error exposed %q", secret)
				}
			}
		})
	}
}

func TestMultipartLoggingAndCleanup(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	t.Setenv("TMP", temp)
	t.Setenv("TEMP", temp)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("field", "FORM_SECRET"); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("upload", "example.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, strings.Repeat("x", (1<<20)+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	server := newServer(config{maxBodyBytes: int64(body.Len()), logBody: true}, log.New(&logs, "", 0))
	req := httptest.NewRequest(http.MethodPost, "/?query=QUERY_SECRET", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	for _, want := range []string{"req.Body:", "FORM_SECRET", "field:[FORM_SECRET]", "query:[QUERY_SECRET]", "req.MultipartForm:"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("multipart log lacks %q", want)
		}
	}
	if req.MultipartForm == nil || len(req.MultipartForm.File["upload"]) != 1 {
		t.Fatal("multipart upload was not parsed")
	}
	uploaded, err := req.MultipartForm.File["upload"][0].Open()
	if err == nil {
		uploaded.Close()
		t.Fatal("multipart temp file was not removed")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("opening cleaned temp file: %v", err)
	}
	entries, err := os.ReadDir(temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("multipart temp files remain: %v", entries)
	}
}
