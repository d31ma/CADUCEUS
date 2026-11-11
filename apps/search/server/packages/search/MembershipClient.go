package search

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type MembershipClient struct {
	messagingUrl string
	channelsUrl  string
	http         *http.Client
}

func NewMembershipClient(messagingUrl string, channelsUrl string, client *http.Client) MembershipClient {
	return MembershipClient{messagingUrl: strings.TrimSuffix(messagingUrl, "/"), channelsUrl: strings.TrimSuffix(channelsUrl, "/"), http: client}
}

func (client MembershipClient) Readable(authorization string) (map[string]bool, error) {
	readable := map[string]bool{}
	err := client.addConversations(authorization, readable)
	if err != nil {
		return nil, err
	}
	err = client.addChannels(authorization, readable)
	if err != nil {
		return nil, err
	}
	return readable, nil
}

func (client MembershipClient) addConversations(authorization string, readable map[string]bool) error {
	cursor, err := client.addConversationPage(authorization, "", readable)
	for pages := 1; err == nil && cursor != "" && pages < client.maxPages(); pages++ {
		cursor, err = client.addConversationPage(authorization, cursor, readable)
	}
	return err
}

func (client MembershipClient) addConversationPage(authorization string, cursor string, readable map[string]bool) (string, error) {
	page := ConversationPage{}
	err := client.read(client.conversationsAddress(cursor), authorization, client.messaging(), &page)
	client.addSummaries(page.Items, readable)
	return page.NextCursor, err
}

func (client MembershipClient) addSummaries(items []ConversationSummary, readable map[string]bool) {
	for _, item := range items {
		readable[item.ConversationId] = true
	}
}

func (client MembershipClient) addChannels(authorization string, readable map[string]bool) error {
	channels := []ChannelSummary{}
	err := client.read(client.channelsUrl+"/api/v1/channels", authorization, client.channels(), &channels)
	for _, channel := range channels {
		readable[channel.ConversationId] = true
	}
	return err
}

func (client MembershipClient) conversationsAddress(cursor string) string {
	query := url.Values{"limit": {"100"}}
	client.addCursor(query, cursor)
	return client.messagingUrl + "/api/v1/messaging/conversations?" + query.Encode()
}

func (client MembershipClient) addCursor(query url.Values, cursor string) {
	if cursor != "" {
		query.Set("cursor", cursor)
	}
}

func (client MembershipClient) read(address string, authorization string, service string, target any) error {
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Accept", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return client.decoded(response, service, target)
}

func (client MembershipClient) decoded(response *http.Response, service string, target any) error {
	if response.StatusCode != http.StatusOK {
		return MembershipFailed{Service: service, Status: response.StatusCode}
	}
	return json.NewDecoder(io.LimitReader(response.Body, client.maxAnswerBytes())).Decode(target)
}

func (client MembershipClient) messaging() string {
	return "messaging"
}

func (client MembershipClient) channels() string {
	return "channels"
}

func (client MembershipClient) maxPages() int {
	return 100
}

func (client MembershipClient) maxAnswerBytes() int64 {
	return 4 << 20
}
