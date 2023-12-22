package main

import (
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"gorm.io/datatypes"
)
 
func main() {
	e := echo.New()
	e.Any("/*", func(c echo.Context) error {
		httpReq := c.Request()
		res := &struct{
			RemoteAddr string
			URL string
			Method string
			Header http.Header
			Body datatypes.JSON
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
				res.Body = reqBytes
			}
		}
		return c.JSON(http.StatusOK, res)
	})
	if err := e.Start(":1323"); err != nil {
		log.Fatal(err.Error())
	}
}