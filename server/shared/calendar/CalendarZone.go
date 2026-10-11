import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CalendarZone converts between instants and the wall-clock ("floating")
// times of an IANA time zone, as the JavaScript server did with Intl. A
// floating time is a wall-clock reading stored as if it were UTC
// milliseconds.
type CalendarZone struct{}

// Valid reports whether value names an IANA time zone.
func (zone CalendarZone) Valid(value any) bool {
	text, isText := value.(string)
	return isText && len([]rune(text)) <= zone.longestName() && zone.Location(text) != nil
}

// Location answers the time zone a name loads, or nil.
func (CalendarZone) Location(name string) *time.Location {
	if name == "" || strings.EqualFold(name, "local") || strings.ContainsAny(name, "\x00\\") || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
		return nil
	}
	if strings.EqualFold(name, "utc") {
		return time.UTC
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return nil
	}
	return location
}

// ToFloating answers the wall-clock reading of an instant in a zone.
func (zone CalendarZone) ToFloating(instant int64, name string) int64 {
	return zone.wallEpoch(instant, name)
}

// FromFloating answers the instant a wall-clock reading names in a zone.
// Exact matches cover ordinary and repeated clock times; a clock time that
// a spring-forward skips takes the later reading so the event stays
// visible.
func (zone CalendarZone) FromFloating(floating int64, name string) int64 {
	if name == "UTC" {
		return floating
	}
	day := int64(36 * 60 * 60 * 1000)
	offsets := []int64{}
	for _, probe := range []int64{floating - day, floating, floating + day} {
		offset := zone.offsetAt(probe, name)
		if !zone.contains(offsets, offset) {
			offsets = append(offsets, offset)
		}
	}
	candidates := [][2]int64{}
	for _, offset := range offsets {
		instant := floating - offset
		candidates = append(candidates, [2]int64{instant, zone.wallEpoch(instant, name)})
	}
	exact := [][2]int64{}
	later := [][2]int64{}
	for _, candidate := range candidates {
		if candidate[1] == floating {
			exact = append(exact, candidate)
		}
		if candidate[1] > floating {
			later = append(later, candidate)
		}
	}
	if len(exact) > 0 {
		sort.SliceStable(exact, func(left, right int) bool { return exact[left][0] < exact[right][0] })
		return exact[0][0]
	}
	if len(later) > 0 {
		sort.SliceStable(later, func(left, right int) bool {
			if later[left][1] != later[right][1] {
				return later[left][1] < later[right][1]
			}
			return later[left][0] < later[right][0]
		})
		return later[0][0]
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return zone.distance(candidates[left][1], floating) < zone.distance(candidates[right][1], floating)
	})
	return candidates[0][0]
}

// ParseDateTime answers an RFC 5545 DATE or DATE-TIME as one absolute ISO
// timestamp (a DATE stays a date). Zoned wall times use the same daylight
// saving choices as recurrence expansion.
func (zone CalendarZone) ParseDateTime(raw string, name string) string {
	value := Security{}.Trim(raw)
	if regexp.MustCompile(`^\d{8}$`).MatchString(value) {
		return CalendarDate{}.FromCompact(value)
	}
	if parts := regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})(Z?)$`).FindStringSubmatch(value); parts != nil {
		numbers := make([]int, 6)
		for index := range numbers {
			numbers[index], _ = strconv.Atoi(parts[index+1])
		}
		floating := time.Date(numbers[0], time.Month(numbers[1]), numbers[2], numbers[3], numbers[4], numbers[5], 0, time.UTC).UnixMilli()
		if parts[7] == "Z" {
			return EpochTime{}.Text(floating)
		}
		if !zone.Valid(name) {
			name = "UTC"
		}
		return EpochTime{}.Text(zone.FromFloating(floating, name))
	}
	parsed := EpochTime{}.Parse(value)
	if math.IsNaN(parsed) {
		return value
	}
	return EpochTime{}.Text(int64(parsed))
}

// offsetAt answers what the JavaScript server measured as a zone's offset
// at an instant: the wall-clock reading minus the instant, plus the
// instant's milliseconds.
func (zone CalendarZone) offsetAt(instant int64, name string) int64 {
	return zone.wallEpoch(instant, name) - instant + zone.milliseconds(instant)
}

// wallEpoch answers the wall-clock reading of an instant, to the second,
// with the instant's own milliseconds.
func (zone CalendarZone) wallEpoch(instant int64, name string) int64 {
	location := zone.Location(name)
	if location == nil {
		location = time.UTC
	}
	wall := time.UnixMilli(instant).In(location)
	return time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), wall.Second(), 0, time.UTC).UnixMilli() + zone.milliseconds(instant)
}

func (CalendarZone) milliseconds(instant int64) int64 {
	return ((instant % 1000) + 1000) % 1000
}

func (CalendarZone) distance(left int64, right int64) int64 {
	if left > right {
		return left - right
	}
	return right - left
}

func (CalendarZone) contains(values []int64, value int64) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func (CalendarZone) longestName() int { return 100 }
