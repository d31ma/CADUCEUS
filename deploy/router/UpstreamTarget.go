package main

import (
	"net/http/httputil"
	"net/url"
)

type UpstreamTarget struct {
	Target *url.URL
}

func (target UpstreamTarget) Rewrite(request *httputil.ProxyRequest) {
	request.SetURL(target.Target)
	request.SetXForwarded()
}
