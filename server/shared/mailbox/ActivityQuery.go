import (
	"errors"
	"math"
)

// ActivityQuery validates GET /activity: the event type, the range or the
// from/to window, the since cursor and the page limit.
type ActivityQuery struct{}

func (ActivityQuery) dayMilliseconds() float64 { return 24 * 60 * 60 * 1000 }

// Options answers the activity options of a query at now, or why the query
// is invalid.
func (query ActivityQuery) Options(values map[string]string, now int64) (TrackingActivityOptions, error) {
	options := TrackingActivityOptions{Type: values["type"]}
	if options.Type == "" {
		options.Type = "all"
	}
	if options.Type != "all" && options.Type != "open" && options.Type != "click" {
		return options, errors.New("type must be all, open, or click")
	}
	ranges := map[string]float64{"7d": 7, "30d": 30, "90d": 90, "all": 0}
	window := values["range"]
	if window == "" {
		window = "30d"
	}
	days, known := ranges[window]
	if !known {
		return options, errors.New("range must be 7d, 30d, 90d, or all")
	}
	from, to := float64(now)-days*query.dayMilliseconds(), float64(now)
	if window == "all" {
		from = 0
	}
	var err error
	if from, err = query.instant(values, "from", from); err != nil {
		return options, err
	}
	if to, err = query.instant(values, "to", to); err != nil {
		return options, err
	}
	if from > to {
		return options, errors.New("from must not be after to")
	}
	options.From, options.To = &from, &to
	if values["since"] != "" {
		since, err := query.instant(values, "since", 0)
		if err != nil {
			return options, err
		}
		options.Since = &since
	}
	limit := float64(50)
	if values["limit"] != "" {
		limit = RecordValues{}.Numeric(values["limit"])
	}
	if math.IsNaN(limit) || limit != math.Trunc(limit) || limit < 1 || limit > 100 {
		return options, errors.New("limit must be an integer from 1 to 100")
	}
	options.Limit = &limit
	return options, nil
}

func (ActivityQuery) instant(values map[string]string, name string, fallback float64) (float64, error) {
	if values[name] == "" {
		return fallback, nil
	}
	parsed := EpochTime{}.Parse(values[name])
	if math.IsNaN(parsed) {
		return 0, errors.New(name + " must be an ISO datetime")
	}
	return parsed, nil
}
