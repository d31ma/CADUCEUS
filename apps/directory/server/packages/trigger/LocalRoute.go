package trigger

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type LocalRoute struct {
	Client *http.Client
}

func (route LocalRoute) Post(applicationJson string, path string, authorization string, tokenFailure error) error {
	if tokenFailure != nil {
		return tokenFailure
	}
	port, err := route.port(applicationJson)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, "http://directory.localhost:"+strconv.Itoa(port)+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", authorization)
	response, err := route.Client.Do(request)
	if err != nil {
		return err
	}
	response.Body.Close()
	return route.outcome(response.StatusCode)
}

func (LocalRoute) port(applicationJson string) (int, error) {
	var snapshot applicationSnapshot
	err := json.Unmarshal([]byte(applicationJson), &snapshot)
	if err != nil {
		return 0, err
	}
	if snapshot.Yon.Server.Port == 0 {
		return 0, errors.New("the application settings name no server port")
	}
	return snapshot.Yon.Server.Port, nil
}

func (LocalRoute) outcome(status int) error {
	if status >= http.StatusMultipleChoices {
		return RouteRefused{Status: status}
	}
	return nil
}
