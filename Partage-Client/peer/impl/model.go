package impl

type UserData struct {
	Username      string
	UserID        string
	FollowerCount int
	CanBeEndorsed bool
}

type Reaction struct {
	AuthorID     string
	RefContentID string
	ReactionText string
	BlockHash    string
	Timestamp    int64
}

type Post struct {
	AuthorID  string
	ContentID string
	Text      string
	BlockHash string
	Timestamp int64
	Reactions []Reaction
	Comments  []Comment
}

type Comment struct {
	AuthorID     string
	ContentID    string
	Text         string
	RefContentID string
	BlockHash    string
	Timestamp    int64
}
