package main

type HttpEvent struct {
	Version               string            `json:"version"`
	RouteKey              string            `json:"routeKey"`
	RawPath               string            `json:"rawPath"`
	RawQueryString        string            `json:"rawQueryString"`
	Cookies               []string          `json:"cookies,omitempty"`
	Headers               map[string]string `json:"headers"`
	QueryStringParameters map[string]string `json:"queryStringParameters,omitempty"`
	RequestContext        EventContext      `json:"requestContext"`
	Body                  string            `json:"body"`
	IsBase64Encoded       bool              `json:"isBase64Encoded"`
}
