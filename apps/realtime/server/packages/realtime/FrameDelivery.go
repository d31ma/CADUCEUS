package realtime

import "errors"

type FrameDelivery struct {
	Pushes ConnectionGateway
}

func (delivery FrameDelivery) Reached(connections []Connection, frame any) (bool, error) {
	reached := false
	for _, connection := range connections {
		sent, err := delivery.sent(connection, frame)
		if err != nil {
			return false, err
		}
		reached = reached || sent
	}
	return reached, nil
}

func (delivery FrameDelivery) Split(groups map[string][]Connection, keys []string, frame any) (DeliverySplit, error) {
	split := DeliverySplit{Reached: []string{}, Missed: []string{}}
	for _, key := range keys {
		reached, err := delivery.Reached(groups[key], frame)
		if err != nil {
			return DeliverySplit{}, err
		}
		split = split.With(key, reached)
	}
	return split, nil
}

func (delivery FrameDelivery) sent(connection Connection, frame any) (bool, error) {
	err := delivery.Pushes.Send(connection.ConnectionId, frame)
	if errors.As(err, &Gone{}) {
		return false, nil
	}
	return err == nil, err
}
