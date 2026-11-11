package main

type EventContext struct {
	AccountId    string      `json:"accountId"`
	ApiId        string      `json:"apiId"`
	DomainName   string      `json:"domainName"`
	DomainPrefix string      `json:"domainPrefix"`
	Http         EventMethod `json:"http"`
	RequestId    string      `json:"requestId"`
	RouteKey     string      `json:"routeKey"`
	Stage        string      `json:"stage"`
	Time         string      `json:"time"`
	TimeEpoch    int64       `json:"timeEpoch"`
}
