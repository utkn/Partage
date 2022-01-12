package impl

type Client struct {
	Self UserData
}

func (c *Client) GetUserData(userID string) UserData {
	return UserData{}
}

func (c *Client) GetPosts(userIDs []string, minTime int64, maxTime int64) []Post {
	return nil
}

func (c *Client) GetComments(postID string) []Comment {
	return nil
}

func (c *Client) PostText(content string) bool {
	return false
}

func (c *Client) PostComment(postID string) bool {
	return false
}

func (c *Client) ReactToPost(refContentID string) bool {
	return false
}

func (c *Client) UndoReaction(reactionBlockHash string) bool {
	return false
}

func (c *Client) FollowUser(userID string) bool {
	return false
}

func (c *Client) UnfollowUser(userID string) bool {
	return false
}

func (c *Client) RequestEndorsement() bool {
	return false
}

func (c *Client) EndorseUser(userID string) bool {
	return false
}
