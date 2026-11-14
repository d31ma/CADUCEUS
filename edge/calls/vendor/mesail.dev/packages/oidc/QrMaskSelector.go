package oidc

type QrMaskSelector struct{}

func (selector QrMaskSelector) best(matrix QrMatrix) QrMatrix {
	mask := QrMask{}
	best := selector.candidate(matrix, 0)
	for number := 1; number < mask.count(); number++ {
		best = selector.lower(best, selector.candidate(matrix, number))
	}
	return best.matrix
}

func (QrMaskSelector) candidate(matrix QrMatrix, number int) QrCandidate {
	masked := matrix.clone()
	QrMask{}.apply(masked, number)
	QrFormatInformation{}.drawFormat(masked, number)
	return QrCandidate{matrix: masked, score: QrPenalty{}.score(masked)}
}

func (QrMaskSelector) lower(best QrCandidate, candidate QrCandidate) QrCandidate {
	if candidate.score < best.score {
		return candidate
	}
	return best
}
