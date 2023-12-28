# Hello World

## Install

```sh
$ go install github.com/w00j00ng/goproj/helloworld@latest
```

## Run

```sh
$ helloworld

   ____    __
  / __/___/ /  ___
 / _// __/ _ \/ _ \
/___/\__/_//_/\___/ v4.11.4
High performance, minimalist Go web framework
https://echo.labstack.com
____________________________________O/_______
                                    O\
⇨ http server started on [::]:1323
```

## Test

### Client

```sh
$ curl localhost:1323
OK
```

### Server Log

```
2023/12/28 18:25:49 req.RemoteAddr: [::1]:55374
2023/12/28 18:25:49 req.URL: /
2023/12/28 18:25:49 req.Method: GET
2023/12/28 18:25:49 req.Header: map[Accept:[*/*] User-Agent:[curl/8.4.0]]
2023/12/28 18:25:49 req.Body:
2023/12/28 18:25:49 req.Form: map[]
2023/12/28 18:25:49 req.MultipartForm: <nil>
```