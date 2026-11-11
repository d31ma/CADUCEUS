package blobs

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type S3Store struct {
	settings S3Settings
	client   *http.Client
	now      func() time.Time
}

func NewS3Store(settings S3Settings, client *http.Client, now func() time.Time) S3Store {
	return S3Store{settings: settings, client: client, now: now}
}

func (store S3Store) SignUpload(key Key, upload UploadTerms, expiresAt time.Time) (string, error) {
	target, err := store.objectUrl(store.settings.PublicUrl(), key, url.Values{})
	if err != nil {
		return "", err
	}
	headers := http.Header{}
	headers.Set("Content-Length", strconv.FormatInt(upload.Size, 10))
	now := store.now()
	return store.signature().Presign(http.MethodPut, target, headers, now, expiresAt.Sub(now)), nil
}

func (store S3Store) SignDownload(key Key, download DownloadTerms, expiresAt time.Time) (string, error) {
	query := url.Values{}
	query.Set("response-content-disposition", Disposition{}.Attachment(download.FileName))
	query.Set("response-content-type", download.MediaType)
	target, err := store.objectUrl(store.settings.PublicUrl(), key, query)
	if err != nil {
		return "", err
	}
	now := store.now()
	return store.signature().Presign(http.MethodGet, target, http.Header{}, now, expiresAt.Sub(now)), nil
}

func (store S3Store) Read(key Key) (io.ReadCloser, error) {
	request, err := store.objectRequest(http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	response, err := store.send(request, store.emptyPayloadHash())
	if err != nil {
		return nil, err
	}
	err = store.missingOrFailure(response, key)
	if err != nil {
		response.Body.Close()
		return nil, err
	}
	return response.Body, nil
}

func (store S3Store) Write(key Key, body io.Reader, size int64) error {
	request, err := store.objectRequest(http.MethodPut, key, io.LimitReader(body, size+1))
	if err != nil {
		return err
	}
	request.ContentLength = size
	response, err := store.send(request, store.unsignedPayload())
	if err != nil {
		return err
	}
	response.Body.Close()
	return store.statusFailure(http.MethodPut, response.StatusCode)
}

func (store S3Store) Copy(from Key, to Key) error {
	err := KeyRules{}.Check(from)
	if err != nil {
		return err
	}
	request, err := store.objectRequest(http.MethodPut, to, nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-Amz-Copy-Source", "/"+store.settings.Bucket+"/"+from.Path())
	response, err := store.send(request, store.emptyPayloadHash())
	if err != nil {
		return err
	}
	response.Body.Close()
	return store.missingOrFailure(response, from)
}

func (store S3Store) Delete(key Key) error {
	request, err := store.objectRequest(http.MethodDelete, key, nil)
	if err != nil {
		return err
	}
	response, err := store.send(request, store.emptyPayloadHash())
	if err != nil {
		return err
	}
	response.Body.Close()
	return store.deleteFailure(response.StatusCode)
}

func (store S3Store) EnsureBucket() error {
	request, err := http.NewRequest(http.MethodPut, store.settings.BaseUrl()+"/"+store.settings.Bucket, nil)
	if err != nil {
		return err
	}
	response, err := store.send(request, store.emptyPayloadHash())
	if err != nil {
		return err
	}
	response.Body.Close()
	return store.bucketFailure(response.StatusCode)
}

func (store S3Store) objectRequest(method string, key Key, body io.Reader) (*http.Request, error) {
	target, err := store.objectUrl(store.settings.BaseUrl(), key, url.Values{})
	if err != nil {
		return nil, err
	}
	return http.NewRequest(method, target.String(), body)
}

func (store S3Store) send(request *http.Request, payloadHash string) (*http.Response, error) {
	store.signature().Authorize(request, payloadHash, store.now())
	return store.client.Do(request)
}

func (store S3Store) objectUrl(base string, key Key, query url.Values) (*url.URL, error) {
	err := KeyRules{}.Check(key)
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	target.Path = "/" + store.settings.Bucket + "/" + key.Path()
	target.RawQuery = AwsEncoding{}.Query(query)
	return target, nil
}

func (store S3Store) missingOrFailure(response *http.Response, key Key) error {
	if response.StatusCode == http.StatusNotFound {
		return NotFound{Path: key.Path()}
	}
	return store.statusFailure(response.Request.Method, response.StatusCode)
}

func (store S3Store) deleteFailure(status int) error {
	if status == http.StatusNotFound {
		return nil
	}
	return store.statusFailure(http.MethodDelete, status)
}

func (store S3Store) bucketFailure(status int) error {
	if status == http.StatusConflict {
		return nil
	}
	return store.statusFailure(http.MethodPut, status)
}

func (S3Store) statusFailure(method string, status int) error {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return S3Failure{Method: method, Status: status}
	}
	return nil
}

func (store S3Store) signature() SignatureV4 {
	return SignatureV4{Settings: store.settings}
}

func (S3Store) emptyPayloadHash() string {
	return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
}

func (S3Store) unsignedPayload() string {
	return "UNSIGNED-PAYLOAD"
}
