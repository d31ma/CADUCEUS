package main

import (
	"context"
	"io"
	"net/http"
	"time"
)

type S3Client struct {
	Endpoint string
	Bucket   string
	Client   *http.Client
	Signer   Signer
	Now      func() time.Time
}

func (client S3Client) Get(context context.Context, key string) (Reply, error) {
	request, err := http.NewRequestWithContext(context, http.MethodGet, client.Endpoint+"/"+client.Bucket+"/"+key, nil)
	if err != nil {
		return Reply{}, err
	}
	client.Signer.Sign(request, nil, client.Now())
	response, err := client.Client.Do(request)
	if err != nil {
		return Reply{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return Reply{}, err
	}
	return client.checked(response, body)
}

func (client S3Client) checked(response *http.Response, body []byte) (Reply, error) {
	switch response.StatusCode {
	case http.StatusOK:
		return Reply{Status: http.StatusOK, Header: client.kept(response.Header), Body: body}, nil
	case http.StatusNotFound, http.StatusForbidden:
		return Reply{}, Problems{}.NotFound()
	default:
		return Reply{}, Problems{}.UpstreamFailed()
	}
}

func (S3Client) kept(header http.Header) http.Header {
	kept := http.Header{}
	for _, name := range []string{"Content-Type", "Content-Length", "Etag", "Last-Modified", "Cache-Control"} {
		kept[name] = header.Values(name)
	}
	return kept
}
