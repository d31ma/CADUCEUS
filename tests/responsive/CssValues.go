//go:build responsive

package responsive

import (
	"math"
	"strconv"
	"strings"
)

func LongestDuration(value string) float64 {
	longest := 0.0
	for _, part := range strings.Split(value, ",") {
		longest = max(longest, durationSeconds(strings.TrimSpace(part)))
	}
	return longest
}

func durationSeconds(part string) float64 {
	if strings.HasSuffix(part, "ms") {
		return parseNumber(strings.TrimSuffix(part, "ms")) / 1000
	}
	return parseNumber(strings.TrimSuffix(part, "s"))
}

func parseNumber(text string) float64 {
	number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0
	}
	return number
}

func PixelValue(value string) float64 {
	return parseNumber(strings.TrimSuffix(strings.TrimSpace(value), "px"))
}

type CssColor struct {
	Luminance float64
	Alpha     float64
	Known     bool
}

func ParseColor(value string) CssColor {
	name, arguments, found := strings.Cut(strings.TrimSpace(value), "(")
	if !found {
		return CssColor{}
	}
	channels, alpha := colorChannels(strings.TrimSuffix(arguments, ")"))
	switch name {
	case "rgb", "rgba":
		return srgbColor(channels, 255, alpha)
	case "color":
		return colorFunction(channels, alpha)
	case "oklch", "oklab":
		return lightnessColor(channels, alpha, math.Pow(percentOrNumber(channels, 1), 3))
	case "lab", "lch":
		return lightnessColor(channels, alpha, cieLightnessToLuminance(percentOrNumber(channels, 100)))
	}
	return CssColor{}
}

func colorChannels(arguments string) ([]string, float64) {
	colorPart, alphaPart, hasSlash := strings.Cut(arguments, "/")
	channels := strings.Fields(strings.ReplaceAll(colorPart, ",", " "))
	alpha := 1.0
	if hasSlash {
		alpha = alphaValue(alphaPart)
	}
	if !hasSlash && len(channels) == 4 {
		alpha = alphaValue(channels[3])
		channels = channels[:3]
	}
	return channels, alpha
}

func alphaValue(text string) float64 {
	text = strings.TrimSpace(text)
	if strings.HasSuffix(text, "%") {
		return parseNumber(strings.TrimSuffix(text, "%")) / 100
	}
	return parseNumber(text)
}

func srgbColor(channels []string, scale float64, alpha float64) CssColor {
	if len(channels) < 3 {
		return CssColor{}
	}
	red := linearChannel(parseNumber(channels[0]) / scale)
	green := linearChannel(parseNumber(channels[1]) / scale)
	blue := linearChannel(parseNumber(channels[2]) / scale)
	return CssColor{Luminance: 0.2126*red + 0.7152*green + 0.0722*blue, Alpha: alpha, Known: true}
}

func colorFunction(channels []string, alpha float64) CssColor {
	if len(channels) < 4 || channels[0] != "srgb" {
		return CssColor{}
	}
	return srgbColor(channels[1:], 1, alpha)
}

func lightnessColor(channels []string, alpha float64, luminance float64) CssColor {
	if len(channels) < 1 {
		return CssColor{}
	}
	return CssColor{Luminance: luminance, Alpha: alpha, Known: true}
}

func percentOrNumber(channels []string, scale float64) float64 {
	if len(channels) < 1 {
		return 0
	}
	if strings.HasSuffix(channels[0], "%") {
		return parseNumber(strings.TrimSuffix(channels[0], "%")) / 100
	}
	return parseNumber(channels[0]) / scale
}

func cieLightnessToLuminance(lightness float64) float64 {
	return math.Pow((lightness*100+16)/116, 3)
}

func linearChannel(channel float64) float64 {
	if channel <= 0.04045 {
		return channel / 12.92
	}
	return math.Pow((channel+0.055)/1.055, 2.4)
}
