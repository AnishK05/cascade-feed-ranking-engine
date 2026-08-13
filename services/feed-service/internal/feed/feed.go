// Package feed contains the internal Feed Service domain types.
package feed

import "time"

// Post is a hydrated feed candidate.
type Post struct {
	ID        int64
	AuthorID  int64
	Content   string
	MediaURL  string
	CreatedAt time.Time
}

// CandidateSet keeps normal and celebrity sources separate until celebrity authors are
// filtered against the viewer's followed-celebrity set.
type CandidateSet struct {
	NormalIDs           []int64
	CelebrityIDs        []int64
	FollowedCelebrities map[int64]struct{}
}

// Signal contains all ranking features loaded for one post.
type Signal struct {
	Likes    int64
	Comments int64
	Affinity float64
}

// RankedPost is a post plus its computed rank score.
type RankedPost struct {
	Post
	Score      float64
	Recency    float64
	Engagement float64
	Affinity   float64
}
