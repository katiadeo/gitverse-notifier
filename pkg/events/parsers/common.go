package parsers

import (
	"encoding/json"
	"fmt"
	"strings"

	"gitverse-notifier/pkg/events"
)

type actorPayload struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Login    string `json:"login"`
	Username string `json:"username"`
	UserName string `json:"userName"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	HTMLURL  string `json:"html_url"`
	URL      string `json:"url"`
}

func (a actorPayload) toActor() events.Actor {
	login := firstNonEmpty(a.Login, a.Username, a.UserName, a.Name)
	name := firstNonEmpty(a.FullName, a.Name, a.Login, a.Username, a.UserName)
	return events.Actor{
		ID:    a.ID,
		Name:  name,
		Login: login,
		Email: a.Email,
		URL:   firstNonEmpty(a.HTMLURL, a.URL),
	}
}

func firstActor(actors ...actorPayload) events.Actor {
	for _, actor := range actors {
		if parsed := actor.toActor(); parsed != (events.Actor{}) {
			return parsed
		}
	}
	return events.Actor{}
}

type repositoryPayload struct {
	FullName       string `json:"fullName"`
	FullNameSnake  string `json:"full_name"`
	Name           string `json:"name"`
	OwnerName      string `json:"ownerName"`
	OwnerNameSnake string `json:"owner_name"`
	Owner          struct {
		Login    string `json:"login"`
		Name     string `json:"name"`
		Username string `json:"username"`
		UserName string `json:"userName"`
	} `json:"owner"`
}

func (r repositoryPayload) repositoryName() string {
	if name := firstNonEmpty(r.FullName, r.FullNameSnake); name != "" {
		return name
	}
	owner := firstNonEmpty(r.OwnerName, r.OwnerNameSnake, r.Owner.Login, r.Owner.Username, r.Owner.UserName, r.Owner.Name)
	if owner != "" && strings.TrimSpace(r.Name) != "" {
		return owner + "/" + strings.TrimSpace(r.Name)
	}
	return ""
}

type commonPayload struct {
	Action     string            `json:"action"`
	Ref        string            `json:"ref"`
	Repository repositoryPayload `json:"repository"`
	Repo       repositoryPayload `json:"repo"`
	Sender     actorPayload      `json:"sender"`
	Pusher     actorPayload      `json:"pusher"`
}

func fillCommon(event *events.Event, body []byte) error {
	if len(body) == 0 {
		return nil
	}

	var payload commonPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse event json: %w", err)
	}

	event.Action = payload.Action
	event.Ref = payload.Ref
	event.Branch = events.BranchName(payload.Ref)
	event.Repository = firstNonEmpty(payload.Repository.repositoryName(), payload.Repo.repositoryName())
	event.Sender = payload.Sender.toActor()

	if event.Sender == (events.Actor{}) {
		event.Sender = payload.Pusher.toActor()
	}

	return nil
}

func requestedReviewers(eventType events.EventType, payload pullRequestPayload, pr pullRequestBodyPayload) []events.Actor {
	if full := uniqueActors(collectRequestedReviewers(payload, pr)...); len(full) > 0 {
		return full
	}
	if eventType == events.PullRequestReviewRequested || eventType == events.PullRequestReviewRequestRemoved {
		if reviewer := firstActor(payload.RequestedReviewer, payload.RequestedReviewerCamel); reviewer != (events.Actor{}) {
			return []events.Actor{reviewer}
		}
	}
	return nil
}

func reviewersExact(payload pullRequestPayload, pr pullRequestBodyPayload) bool {
	return len(collectRequestedReviewers(payload, pr)) > 0
}

func collectRequestedReviewers(payload pullRequestPayload, pr pullRequestBodyPayload) []actorPayload {
	var list []actorPayload
	list = append(list, payload.RequestedReviewers...)
	list = append(list, payload.RequestedReviewersCamel...)
	list = append(list, pr.RequestedReviewers...)
	list = append(list, pr.RequestedReviewersCamel...)
	list = append(list, pr.Reviewers...)
	return list
}

func uniqueActors(list ...actorPayload) []events.Actor {
	out := make([]events.Actor, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, item := range list {
		actor := item.toActor()
		if actor == (events.Actor{}) {
			continue
		}
		key := actor.Name
		if actor.ID != 0 {
			key = fmt.Sprintf("id:%d", actor.ID)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, actor)
	}
	return out
}

type flexInt int

func (n *flexInt) UnmarshalJSON(data []byte) error {
	data = bytesTrimJSON(data)
	if len(data) == 0 || string(data) == "null" || string(data) == `""` {
		*n = 0
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			*n = 0
			return nil
		}
		var v int
		if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &v); err != nil {
			*n = 0
			return nil
		}
		*n = flexInt(v)
		return nil
	}
	var v int
	if err := json.Unmarshal(data, &v); err == nil {
		*n = flexInt(v)
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		*n = 0
		return nil
	}
	*n = flexInt(int(f))
	return nil
}

func (n flexInt) Int() int { return int(n) }

func bytesTrimJSON(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
