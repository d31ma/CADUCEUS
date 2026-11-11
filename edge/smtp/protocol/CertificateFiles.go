package protocol

import (
	"crypto/tls"
	"os"
	"sync"
)

type CertificateFiles struct {
	certificatePath string
	keyPath         string
	mutex           sync.Mutex
	stamp           FileStamp
	certificate     *tls.Certificate
}

func NewCertificateFiles(certificatePath string, keyPath string) *CertificateFiles {
	return &CertificateFiles{certificatePath: certificatePath, keyPath: keyPath}
}

func (files *CertificateFiles) TlsSettings() *tls.Config {
	return &tls.Config{GetCertificate: files.Certificate, MinVersion: tls.VersionTLS12}
}

func (files *CertificateFiles) Certificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	files.mutex.Lock()
	defer files.mutex.Unlock()
	stamp, err := files.currentStamp()
	if err != nil {
		return nil, err
	}
	return files.loaderFor(stamp)(stamp)
}

func (files *CertificateFiles) currentStamp() (FileStamp, error) {
	certificateInfo, err := os.Stat(files.certificatePath)
	if err != nil {
		return FileStamp{}, err
	}
	keyInfo, err := os.Stat(files.keyPath)
	if err != nil {
		return FileStamp{}, err
	}
	return FileStamp{CertificateTime: certificateInfo.ModTime(), KeyTime: keyInfo.ModTime()}, nil
}

func (files *CertificateFiles) loaderFor(stamp FileStamp) func(FileStamp) (*tls.Certificate, error) {
	if files.certificate != nil && files.stamp.CertificateTime.Equal(stamp.CertificateTime) && files.stamp.KeyTime.Equal(stamp.KeyTime) {
		return files.cached
	}
	return files.load
}

func (files *CertificateFiles) cached(_ FileStamp) (*tls.Certificate, error) {
	return files.certificate, nil
}

func (files *CertificateFiles) load(stamp FileStamp) (*tls.Certificate, error) {
	pair, err := tls.LoadX509KeyPair(files.certificatePath, files.keyPath)
	if err != nil {
		return nil, err
	}
	files.certificate = &pair
	files.stamp = stamp
	return files.certificate, nil
}
