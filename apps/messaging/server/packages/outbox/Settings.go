package outbox

import "mesail.dev/packages/events"

type Settings struct {
	Producer    string        `json:"producer"`
	Subscribers []Subscriber  `json:"subscribers"`
	Retry       RetrySettings `json:"retry"`
}

func (settings Settings) consumersOf(eventType string) []string {
	return settings.listed(events.Catalog{}.ConsumersOf(eventType), settings.subscriberMap())
}

func (settings Settings) subscriberMap() map[string]Subscriber {
	byName := map[string]Subscriber{}
	for _, subscriber := range settings.Subscribers {
		byName[subscriber.Name] = subscriber
	}
	return byName
}

func (settings Settings) listed(names []string, byName map[string]Subscriber) []string {
	kept := []string{}
	for _, name := range names {
		kept = append(kept, settings.ifListed(name, byName)...)
	}
	return kept
}

func (Settings) ifListed(name string, byName map[string]Subscriber) []string {
	_, listed := byName[name]
	if !listed {
		return []string{}
	}
	return []string{name}
}
