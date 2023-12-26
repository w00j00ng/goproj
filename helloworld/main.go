package main

import (
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
)
 
func main() {
	e := echo.New()
	e.Any("/*", func(c echo.Context) error {
		httpReq := c.Request()
		req := &struct{
			RemoteAddr string
			URL string
			Method string
			Header http.Header
			Body []byte
			Form url.Values
			MultipartForm *multipart.Form
		}{
			RemoteAddr: httpReq.RemoteAddr,
			URL: httpReq.URL.String(),
			Method: httpReq.Method,
			Header: httpReq.Header,
			Form: httpReq.Form,
			MultipartForm: httpReq.MultipartForm,
		}
		if httpReq.Body != nil {
			reqBytes, err := io.ReadAll(httpReq.Body)
			if err == nil {
				req.Body = reqBytes
			}
		}
		log.Printf("req.RemoteAddr: %v", req.RemoteAddr)
		log.Printf("req.URL: %v", req.URL)
		log.Printf("req.Method: %v", req.Method)
		log.Printf("req.Header: %v", req.Header)
		log.Printf("req.Body: %v", string(req.Body))
		log.Printf("req.Form: %v", req.Form)
		log.Printf("req.MultipartForm: %v", req.MultipartForm)
		return c.String(http.StatusOK, "OK")
	})
	if err := e.Start(":1323"); err != nil {
		log.Fatal(err.Error())
	}
}
