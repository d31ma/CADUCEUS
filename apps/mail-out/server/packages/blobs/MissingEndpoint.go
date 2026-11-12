package blobs

type MissingEndpoint struct{}

func (MissingEndpoint) Error() string {
	return "the S3 blob store needs an endpoint setting: Mesail has no default S3 host, so name the S3-compatible store to use or choose the local adapter"
}
