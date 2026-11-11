package mailout

import "mesail.dev/packages/blobs"

func blobKey(domainId string, name string) blobs.Key {
	return blobs.Key{DomainId: domainId, Name: name}
}
