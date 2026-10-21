# Hello World

An Echo HTTP request inspector. Every successfully read request returns `200 OK` with the body `OK`.

## Install

Requires Go 1.27.3 or newer, including the standard library security fixes.

```sh
go install github.com/w00j00ng/goproj/helloworld@latest
```

## Run

```sh
helloworld
curl http://127.0.0.1:1323/hello
# OK
```

The default listener is `127.0.0.1:1323`, accessible only from the local machine. Use an explicit address to allow remote access:

```sh
helloworld --addr=0.0.0.0:1323
```

| Flag | Default | Purpose |
| --- | --- | --- |
| `--addr` | `127.0.0.1:1323` | HTTP listen address |
| `--max-body-bytes` | `1048576` (1 MiB) | Maximum request body size; must be a positive integer |
| `--log-body` | `false` | Log raw request bodies and parsed form values |

The body limit applies to every request, including requests without `Content-Length`. A body exactly at the limit is accepted; larger bodies receive `413`. Body read failures receive `400`. With `--log-body`, malformed forms or query encoding also receive `400`. Rejected bodies and underlying error details are not logged.

## Logs and privacy

Default logs include the remote address, method, path (without the query string), and headers. Request bodies and parsed forms are omitted. `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, and `X-API-Key` values are replaced with `[REDACTED]`, regardless of header-name casing. This does not modify the request headers.

Other header values and URL paths remain visible, so avoid putting secrets in them.

To inspect raw bodies and forms explicitly:

```sh
helloworld --log-body --max-body-bytes=2097152
curl -d 'name=example' 'http://127.0.0.1:1323/hello?source=local'
```

Opt-in logs include the raw body, parsed form values, and query values. URL-encoded body fields follow Go ParseForm semantics and are parsed for POST, PUT and PATCH; bodies on other methods are still available in the raw body log. Multipart forms are parsed whenever their content type is supplied. Uploaded file content is also present in the raw multipart body. These logs can contain passwords, tokens, and personal information; use this option only for requests whose contents may be logged. Sensitive headers remain redacted. Multipart temporary files are removed after handling the request.

## Test

```sh
go test ./...
```
