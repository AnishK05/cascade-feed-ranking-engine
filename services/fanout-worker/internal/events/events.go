package events

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrPermanent = errors.New("permanent event error")

type PostCreated struct {
	EventType       string `json:"eventType"`
	PostID          int64  `json:"postId"`
	AuthorID        int64  `json:"authorId"`
	CreatedAtUnixMs int64  `json:"createdAtUnixMs"`
}

type PostDeleted struct {
	EventType       string `json:"eventType"`
	PostID          int64  `json:"postId"`
	AuthorID        int64  `json:"authorId"`
	DeletedAtUnixMs int64  `json:"deletedAtUnixMs"`
}

type FollowCreated struct {
	EventType       string `json:"eventType"`
	FollowerID      int64  `json:"followerId"`
	FolloweeID      int64  `json:"followeeId"`
	CreatedAtUnixMs int64  `json:"createdAtUnixMs"`
}

type FollowDeleted struct {
	EventType       string `json:"eventType"`
	FollowerID      int64  `json:"followerId"`
	FolloweeID      int64  `json:"followeeId"`
	DeletedAtUnixMs int64  `json:"deletedAtUnixMs"`
}

type envelope struct {
	EventType string `json:"eventType"`
}

func ParsePost(payload []byte) (any, error) {
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, permanent("decode post event", err)
	}
	switch env.EventType {
	case "PostCreated":
		var event PostCreated
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, permanent("decode PostCreated", err)
		}
		if event.PostID <= 0 || event.AuthorID <= 0 || event.CreatedAtUnixMs <= 0 {
			return nil, permanent("validate PostCreated", errors.New("IDs and createdAtUnixMs must be positive"))
		}
		return event, nil
	case "PostDeleted":
		var event PostDeleted
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, permanent("decode PostDeleted", err)
		}
		if event.PostID <= 0 || event.AuthorID <= 0 || event.DeletedAtUnixMs <= 0 {
			return nil, permanent("validate PostDeleted", errors.New("IDs and deletedAtUnixMs must be positive"))
		}
		return event, nil
	default:
		return nil, permanent("post event", fmt.Errorf("unknown eventType %q", env.EventType))
	}
}

func ParseFollow(payload []byte) (any, error) {
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, permanent("decode follow event", err)
	}
	switch env.EventType {
	case "FollowCreated":
		var event FollowCreated
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, permanent("decode FollowCreated", err)
		}
		if event.FollowerID <= 0 || event.FolloweeID <= 0 || event.CreatedAtUnixMs <= 0 {
			return nil, permanent("validate FollowCreated", errors.New("IDs and createdAtUnixMs must be positive"))
		}
		return event, nil
	case "FollowDeleted":
		var event FollowDeleted
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, permanent("decode FollowDeleted", err)
		}
		if event.FollowerID <= 0 || event.FolloweeID <= 0 || event.DeletedAtUnixMs <= 0 {
			return nil, permanent("validate FollowDeleted", errors.New("IDs and deletedAtUnixMs must be positive"))
		}
		return event, nil
	default:
		return nil, permanent("follow event", fmt.Errorf("unknown eventType %q", env.EventType))
	}
}

func permanent(operation string, err error) error {
	return fmt.Errorf("%w: %s: %v", ErrPermanent, operation, err)
}
