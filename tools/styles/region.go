package main

import (
	"errors"
	"strings"
)

const (
	regionStart = "/* @tachyon-generated-tailwind:start */"
	regionEnd   = "/* @tachyon-generated-tailwind:end */"
)

func withRegion(original, generated string) (string, error) {
	authored, found, err := authoredPart(original)
	if err != nil {
		return "", err
	}
	if generated == "" {
		return withoutRegion(original, authored, found), nil
	}
	region := regionStart + "\n" + generated + "\n" + regionEnd + "\n"
	if authored == "" {
		return region, nil
	}
	return authored + "\n\n" + region, nil
}

func withoutRegion(original, authored string, found bool) string {
	if !found {
		return original
	}
	if authored == "" {
		return ""
	}
	return authored + "\n"
}

func authoredPart(original string) (string, bool, error) {
	starts := strings.Count(original, regionStart)
	ends := strings.Count(original, regionEnd)
	if starts == 0 && ends == 0 {
		return trimEnd(original), false, nil
	}
	start := strings.Index(original, regionStart)
	end := strings.Index(original, regionEnd)
	if starts != 1 || ends != 1 || end < start {
		return "", false, errors.New("the generated Tailwind region is incomplete or repeated")
	}
	return trimEnd(original[:start] + original[end+len(regionEnd):]), true, nil
}

func trimEnd(text string) string {
	return strings.TrimRight(text, " \t\r\n")
}
