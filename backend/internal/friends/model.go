package friends

import "time"

// Relation describes how the viewer relates to another user. Used in user
// search results so the frontend can render the right action button.
type Relation string

const (
	RelationNone     Relation = "none"
	RelationSelf     Relation = "self"
	RelationFriend   Relation = "friend"
	RelationIncoming Relation = "incoming_request"
	RelationOutgoing Relation = "outgoing_request"
)

// Friend is the viewer-facing representation of a friend.
type Friend struct {
	Username    string    `db:"username"     json:"username"`
	DisplayName string    `db:"display_name" json:"displayName"`
	Since       time.Time `db:"since"        json:"since"`
}

// Request is one pending friend request, rendered from the viewer's
// perspective. The "username" field is the *other* party: the sender on an
// incoming request, the recipient on an outgoing one.
type Request struct {
	Username    string    `db:"username"     json:"username"`
	DisplayName string    `db:"display_name" json:"displayName"`
	Message     string    `db:"message"      json:"message,omitempty"`
	CreatedAt   time.Time `db:"created_at"   json:"createdAt"`
}

// SearchResult is one entry returned by GET /users/search.
type SearchResult struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	AvatarURL   string   `json:"avatarUrl,omitempty"`
	Relation    Relation `json:"relation"`
}
