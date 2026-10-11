
// ViewportMeasured says whether the window is phone-sized. CalendarBrowserService
// publishes it, retained, at start and after every resize.
type ViewportMeasured struct {
	Narrow bool `json:"narrow"`
}
