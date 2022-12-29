package notify

import (
	"bytes"
	"encoding/json"
	"es-backup/config"
	"log"
	"net/http"
)

type Mattermost struct {
	p *config.Params
}

type Payload struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	Props     Props  `json:"props"`
}
type Attachments struct {
	Pretext string `json:"pretext"`
	Text    string `json:"text"`
}
type Props struct {
	Attachments []Attachments `json:"attachments"`
}

func NewMattermost(params *config.Params) (m *Mattermost) {
	m = &Mattermost{
		p: params,
	}
	return
}

func (m *Mattermost) Notify(message string, pretext string, text string, isError bool) {
	if !m.p.Notify.Mattermost.Enabled {
		return
	}

	if m.p.Hostname != "" {
		message = "**[" + m.p.Hostname + "]** " + message
	}

	var channelID, apiToken, url string

	if isError {
		channelID = m.p.Notify.Mattermost.Error.ChannelId
		apiToken = m.p.Notify.Mattermost.Error.ApiToken
		url = m.p.Notify.Mattermost.Error.Url+"/api/v4/posts"
	} else {
		channelID = m.p.Notify.Mattermost.Info.ChannelId
		apiToken = m.p.Notify.Mattermost.Info.ApiToken
		url = m.p.Notify.Mattermost.Info.Url+"/api/v4/posts"
	}

	data := Payload{
		ChannelID: channelID,
		Message:   message,
	}

	if text != "" {
		formattedText := "``` yaml\n" + text + "\n```"

		data.Props.Attachments = append(
			data.Props.Attachments,
			Attachments{
				Pretext: pretext,
				Text:    formattedText,
			},
		)
	}

	payloadBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshalling message payload for Mattermost: %q", err.Error())
		return
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		log.Printf("Error creating POST request for Mattermost: %q", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error sending POST request to Mattermost: %q", err.Error())
		return
	}
	statusOK := res.StatusCode >= 200 && res.StatusCode < 300
	if !statusOK {
		log.Printf("Non-OK HTTP status sending POST request to Mattermost: %q", res.Status)
		return
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			log.Printf("Error closing response body from Mattermost: %q", err.Error())
			return
		}
	}()
}
