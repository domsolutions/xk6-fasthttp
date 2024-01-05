# xk6-fasthttp

[![Build status](https://github.com/domsolutions/xk6-fasthttp/actions/workflows/go.yml/badge.svg)](https://github.com/domsolutions/xk6-fasthttp/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/domsolutions/xk6-fasthttp)](https://goreportcard.com/report/github.com/domsolutions/xk6-fasthttp)
[![GoDoc](https://godoc.org/github.com/domsolutions/xk6-fasthttp?status.svg)](http://godoc.org/github.com/domsolutions/xk6-fasthttp)

The xk6-fasthttp project is a k6 [extension](https://k6.io/docs/extensions/guides/what-are-k6-extensions/) that enables k6 users to send a higher RPS (request per second) for HTTP/1.1 than with the standard k6 build. It achieves this by using the [fasthttp](https://github.com/valyala/fasthttp) library which has lots of memory/CPU optimization and by optimizing the library which checks the response status.

This is intended for users who wish to stress test a HTTP/1.1 server with a higher RPS than normally possible with k6.

Note this extension **only supports HTTP/1.1.** 

## Features
- Increased RPS on HTTPS connections of **74%**
- Increased RPS on HTTP connections of **75%**
- Ability to stream files from disk with `FileStream` so k6 doesn't run out of memory
- Supports JSON/DOM manipulation same as `k6/http`

## Benchmarks

Using the standard `k6/http` library:

```javascript
import http from 'k6/http';
import { check } from 'k6';

export const options = {
	insecureSkipTLSVerify: true,
};

// Simulated user behavior
export default function () {
	let res = http.post("https://localhost:8080");
	check(res, { 'status was 200': (r) => r.status == 200 });
}
```

Results:

```shell
./k6 run -u 250 -d 10s -q ./std-k6.js 

     ✓ status was 200

     checks.........................: 100.00% ✓ 65683       ✗ 0    
     data_received..................: 9.9 MB  988 kB/s
     data_sent......................: 2.6 MB  260 kB/s
     http_req_blocked...............: avg=2.47ms   min=650ns    med=1.22µs   max=1.27s    p(90)=1.55µs   p(95)=1.68µs  
     http_req_connecting............: avg=8.39µs   min=0s       med=0s       max=26ms     p(90)=0s       p(95)=0s      
     http_req_duration..............: avg=33.81ms  min=427.75µs med=25.52ms  max=854.5ms  p(90)=67.87ms  p(95)=86.46ms 
       { expected_response:true }...: avg=33.81ms  min=427.75µs med=25.52ms  max=854.5ms  p(90)=67.87ms  p(95)=86.46ms 
     http_req_failed................: 0.00%   ✓ 0           ✗ 65683
     http_req_receiving.............: avg=9.06ms   min=35.49µs  med=4.06ms   max=135.34ms p(90)=22.82ms  p(95)=35.3ms  
     http_req_sending...............: avg=362.49µs min=71.45µs  med=108.92µs max=166.3ms  p(90)=155.04µs p(95)=246.26µs
     http_req_tls_handshaking.......: avg=2.45ms   min=0s       med=0s       max=1.27s    p(90)=0s       p(95)=0s      
     http_req_waiting...............: avg=24.38ms  min=0s       med=18.07ms  max=850.99ms p(90)=50.25ms  p(95)=63.8ms  
     http_reqs......................: 65683   6554.766172/s
     iteration_duration.............: avg=37.55ms  min=705.75µs med=26.87ms  max=1.37s    p(90)=70.37ms  p(95)=90.76ms 
     iterations.....................: 65683   6554.766172/s
     vus............................: 250     min=250       max=250
     vus_max........................: 250     min=250       max=250


```

Using `k6/x/fasthttp` library:

```javascript
import { Request, Client, checkstatus } from "k6/x/fasthttp"

const config = {
	max_conns_per_host: 1,
	tls_config: {
		insecure_skip_verify: true,
	}
}


const client = new Client(config);

let req = new Request("https://localhost:8080/");


// Simulated user behavior
export default function () {
		let res = client.get(req);
		checkstatus(200, res);
}

```

Results:

```shell
./k6 run -u 250 -d 10s -q ./file-stream-upload.js 

     ✓ check status is 200

     checks...............: 100.00% ✓ 117129       ✗ 0    
     data_received........: 0 B     0 B/s
     data_sent............: 0 B     0 B/s
     http_req_duration....: avg=20.44ms min=223.18µs med=12.93ms max=1.38s p(90)=41.39ms p(95)=55.58ms
     http_reqs............: 116879  11671.351065/s
     iteration_duration...: avg=21.17ms min=311.78µs med=13.5ms  max=1.38s p(90)=43.01ms p(95)=57.71ms
     iterations...........: 117129  11696.315668/s
     vus..................: 250     min=250        max=250
     vus_max..............: 250     min=250        max=250


```

Can see improved RPS of `11671.351065/s` compared to `6554.766172/s`

## Streaming

K6 currently doesn't allow streaming files from disk when uploading. This extension introduces this feature which can be useful to keep the memory footprint low as the whole file does not need loading into memory first and allows for faster uploads:


```javascript
import { Request, Client, FileStream, checkstatus } from "k6/x/fasthttp"

const config = {
	max_conns_per_host: 1,
	tls_config: {
		insecure_skip_verify: true,
	}
}

const client = new Client(config);

const binFile = new FileStream('/home/john/my-large-data.bin');


let req = new Request("https://localhost:8080/", {
	body : binFile
});


// Simulated user behavior
export default function () {
		let res = client.post(req);
		checkstatus(200, res);
}
```

## Install

1. Install xk6:
```shell
go install go.k6.io/xk6/cmd/xk6@latest
```
2. Build the binary with the latest extension:
```shell
xk6 build --with github.com/domsolutions/xk6-fasthttp@latest
```

## Configuration

### Client

The `Client` object takes the following configuration options in its constructor with default values as below:

```javascript
{
  // timeout for attempting connection
  "dial_timeout": 5, 
  // optional proxy to connect to i.e. "username:password@localhost:9050"    
  "proxy": "",
  // max connection duration, 0 is unlimited
  "max_conn_duration": 0,
  // user agent to send in HTTP header
  "user_agent": "",
  // Per-connection buffer size for responses' reading. 0 is unlimited
  "read_buffer_size": 0,
  // Per-connection buffer size for requests' writing.
  "write_buffer_size": 0,
  // Maximum duration for full response reading (including body). 0 is unlimited
  "read_timeout": 0,
  // Maximum duration for full request writing (including body).
  "write_timeout": 0,
  // Maximum number of connections per each host which may be established.
  "max_conns_per_host": 1,
  "tls_config": {
        // skip CA signer verification - useful for localhost testing
        "insecure_skip_verify": false,
        // private key file path for mTLS handshake
        "private_key": "",
        // certificate file path for mTLS handshake
        "certificate": ""    
  }
}
```

### Request

The `Request` object takes the following configuration options in its constructor with default values as below:

```javascript
{
    // whehter to exit with error if a request fails
    "throw": false,
    // disable keeping connection alive between requests
    "disable_keep_alive": false,
    // override the host header
    "host": "",
    // object of HTTP headers
    "headers":{},
    // body to send
    "body": "<FileStream><String>",
    // expected response type: text,binary,none. If none response body will be discarded
    "response_type": "text"
}
```

## Not supported

- The [fasthttp](https://github.com/valyala/fasthttp) library lacks certain observability features which the standard HTTP package has so we lose these metrics:

```shell
     data_received..................: 9.9 MB  988 kB/s
     data_sent......................: 2.6 MB  260 kB/s
     http_req_blocked...............: avg=2.47ms   min=650ns    med=1.22µs   max=1.27s    p(90)=1.55µs   p(95)=1.68µs  
     http_req_connecting............: avg=8.39µs   min=0s       med=0s       max=26ms     p(90)=0s       p(95)=0s      
     http_req_duration..............: avg=33.81ms  min=427.75µs med=25.52ms  max=854.5ms  p(90)=67.87ms  p(95)=86.46ms 
       { expected_response:true }...: avg=33.81ms  min=427.75µs med=25.52ms  max=854.5ms  p(90)=67.87ms  p(95)=86.46ms 
     http_req_failed................: 0.00%   ✓ 0           ✗ 65683
     http_req_receiving.............: avg=9.06ms   min=35.49µs  med=4.06ms   max=135.34ms p(90)=22.82ms  p(95)=35.3ms  
     http_req_sending...............: avg=362.49µs min=71.45µs  med=108.92µs max=166.3ms  p(90)=155.04µs p(95)=246.26µs
     http_req_tls_handshaking.......: avg=2.45ms   min=0s       med=0s       max=1.27s    p(90)=0s       p(95)=0s      
     http_req_waiting...............: avg=24.38ms  min=0s       med=18.07ms  max=850.99ms p(90)=50.25ms  p(95)=63.8ms  
```

In future releases this may become available, will work on creating a PR against [fasthttp](https://github.com/valyala/fasthttp)

- Currently doesn't support cookie jars
- Uncompressing response bodies i.e. gzip

If these features are required, should definitely use the standard `k6/http` package.

## Optimization tips

Create the request object in the `init` context so it doesn't repeatedly get created on every iteration (safe for reuse within VU) i.e:

```javascript
import { Request, Client, checkstatus } from "k6/x/fasthttp"

const config = {
	max_conns_per_host: 1,
	tls_config: {
		insecure_skip_verify: true,
	}
}

const client = new Client();
let req = new Request("http://localhost:8080/");

export default function () {
		let res = client.post(req);
		checkstatus(200, res);
}
```

Also use `checkstatus` to verify the status instead of:

```javascript
import { check } from 'k6';
```

as `checkstatus` doesn't use a closure, so no assertion needed so less CPU cycles.

## Examples

Can find more examples [here](./examples)