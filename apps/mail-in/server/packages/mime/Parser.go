package mime

import "bytes"

type Parser struct{}

func (Parser) MaxSize() int { return 25 << 20 }

func (Parser) MaxHeaderSize() int { return 1 << 20 }

func (Parser) MaxDepth() int { return 16 }

func (Parser) MaxParts() int { return 256 }

func (parser Parser) Headers(raw []byte) (Headers, error) {
	block, err := parser.read(raw)
	if err != nil {
		return Headers{}, err
	}
	return headerFields{}.From(block.header), nil
}

func (parser Parser) Parse(raw []byte) (Message, error) {
	block, err := parser.read(raw)
	if err != nil {
		return Message{}, err
	}
	walker := newWalker(parser, headerFields{}.From(block.header))
	top := newEntity(block.header, bytes.NewReader(block.body), 0)
	return walker.result(walker.walk(top))
}

func (parser Parser) read(raw []byte) (headerBlock, error) {
	err := parser.atMost(len(raw), parser.MaxSize(), TooLarge{})
	if err != nil {
		return headerBlock{}, err
	}
	block, err := headerBlock{}.read(raw)
	if err != nil {
		return headerBlock{}, err
	}
	return block, parser.atMost(block.size, parser.MaxHeaderSize(), TooLarge{})
}

func (parser Parser) depthLimit(depth int) error {
	return parser.atMost(depth, parser.MaxDepth(), TooDeep{})
}

func (parser Parser) partLimit(parts int) error {
	return parser.atMost(parts, parser.MaxParts(), TooManyParts{})
}

func (Parser) atMost(value int, limit int, failure error) error {
	if value > limit {
		return failure
	}
	return nil
}
