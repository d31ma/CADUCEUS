package oidc

type QrCode struct{}

func (QrCode) DataUri(text string) (string, error) {
	data := []byte(text)
	version, err := QrVersionChooser{}.choose(len(data))
	if err != nil {
		return "", err
	}
	codewords := QrBlockInterleaver{}.codewords(QrDataEncoder{}.encode(data, version), version)
	matrix := QrMatrix{}.sized(QrVersionTable{}.size(version))
	QrFunctionPatterns{}.draw(matrix, version)
	QrDataPlacer{}.place(matrix, codewords)
	return QrPicture{}.dataUri(QrMaskSelector{}.best(matrix))
}
