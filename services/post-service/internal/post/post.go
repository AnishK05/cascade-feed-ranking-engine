package post

import "time"

// Post is the service's persistence and cache representation.
type Post struct {
	ID        int64     `json:"id"`
	AuthorID  int64     `json:"authorId"`
	Content   string    `json:"content"`
	MediaURL  string    `json:"mediaUrl,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
