package impl

import (
	"fmt"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/utils"
	"sort"
)

// Client is a useful Partage Client to be used by a frontend.
type Client struct {
	Peer peer.SocialPeer
}

// GetUserData returns the user data associated with the given user id.
func (c *Client) GetUserData(userID string) UserData {
	selfID := c.Peer.GetUserID()
	return NewUserData(selfID, c.Peer.GetUserState(userID))
}

// GetTexts returns the texts with the given filters.
func (c *Client) GetTexts(userIDs []string, minTime int64, maxTime int64) []Text {
	// First, create the filter accordingly.
	filter := content.Filter{
		MaxTime:      maxTime,
		MinTime:      minTime,
		OwnerIDs:     userIDs,
		Types:        []content.Type{content.TEXT},
		RefContentID: "",
	}
	textThings := c.getDownloadableThings(filter, c.downloadText)
	var texts []Text
	for _, t := range textThings {
		texts = append(texts, t.(Text))
	}
	// Sort by the timestamp.
	sort.SliceStable(texts, func(i, j int) bool {
		return texts[i].Timestamp < texts[j].Timestamp
	})
	return texts
}

// GetComments returns the comments associated with the given content id.
func (c *Client) GetComments(contentID string) []Comment {
	// First, create the filter accordingly.
	filter := content.Filter{
		MaxTime:      0,
		MinTime:      0,
		OwnerIDs:     nil,
		Types:        []content.Type{content.COMMENT},
		RefContentID: contentID,
	}
	commentThings := c.getDownloadableThings(filter, c.downloadComment)
	var comments []Comment
	for _, t := range commentThings {
		comments = append(comments, t.(Comment))
	}
	// Sort by the timestamp.
	sort.SliceStable(comments, func(i, j int) bool {
		return comments[i].Timestamp < comments[j].Timestamp
	})
	return comments
}

// GetReactions returns the reactions associated with the given content id.
func (c *Client) GetReactions(contentID string) []Reaction {
	reactionInfos := c.Peer.GetReactions(contentID)
	var reactions []Reaction
	for _, r := range reactionInfos {
		reactions = append(reactions, NewReaction(r))
	}
	return reactions
}

// PostText posts a new text.
func (c *Client) PostText(text string) error {
	postable := content.NewTextPost(c.Peer.GetUserID(), text, utils.Time())
	_, _, err := c.Peer.ShareTextPost(postable)
	return err
}

// PostComment posts a new comment.
func (c *Client) PostComment(comment string, postContentID string) error {
	postable := content.NewCommentPost(c.Peer.GetUserID(), comment, utils.Time(), postContentID)
	_, _, err := c.Peer.ShareCommentPost(postable)
	return err
}

// ReactToPost reacts to the given text/comment content id.
func (c *Client) ReactToPost(reaction content.Reaction, contentID string) error {
	_, err := c.Peer.UpdateFeed(content.CreateReactionMetadata(c.Peer.GetUserID(), reaction, utils.Time(), contentID))
	return err
}

// UndoReaction undoes the reaction made to the content associated with the given content id.
func (c *Client) UndoReaction(contentID string) error {
	// Try to find the latest reaction made by this user for the given user.
	reactions := c.Peer.QueryContents(content.Filter{
		MaxTime:      0,
		MinTime:      0,
		OwnerIDs:     []string{c.Peer.GetUserID()},
		Types:        []content.Type{content.REACTION},
		RefContentID: contentID,
	})
	if len(reactions) == 0 {
		return fmt.Errorf("already unfollowed")
	}
	// Otherwise, try to undo the latest reaction.
	_, err := c.Peer.UpdateFeed(content.CreateUndoMetadata(c.Peer.GetUserID(), utils.Time(), reactions[len(reactions)-1].BlockHash))
	return err
}

// FollowUser follows the user associated with the given user id.
func (c *Client) FollowUser(userID string) error {
	_, err := c.Peer.UpdateFeed(content.CreateFollowUserMetadata(c.Peer.GetUserID(), userID))
	return err
}

// UnfollowUser unfollows the user associated with the given user id.
func (c *Client) UnfollowUser(userID string) error {
	// Try to find the latest follow made by this user for the given user.
	follows := c.Peer.QueryContents(content.Filter{
		MaxTime:  0,
		MinTime:  0,
		OwnerIDs: []string{c.Peer.GetUserID()},
		Types:    []content.Type{content.FOLLOW},
		Data:     content.CreateFollowUserMetadata(c.Peer.GetUserID(), userID).Data,
	})
	if len(follows) == 0 {
		return fmt.Errorf("already unfollowed")
	}
	// Otherwise, try to undo the latest follow action.
	_, err := c.Peer.UpdateFeed(content.CreateUndoMetadata(c.Peer.GetUserID(), utils.Time(), follows[len(follows)-1].BlockHash))
	return err
}

// RequestEndorsement initiates an endorsement request.
func (c *Client) RequestEndorsement() error {
	_, err := c.Peer.UpdateFeed(content.CreateEndorsementRequestMetadata(c.Peer.GetUserID(), utils.Time()))
	return err
}

// EndorseUser endorses the given user.
func (c *Client) EndorseUser(userID string) error {
	_, err := c.Peer.UpdateFeed(content.CreateEndorseUserMetadata(c.Peer.GetUserID(), utils.Time(), userID))
	return err
}

// In case of an error, returns an incomplete Text and an error.
func (c *Client) downloadText(cnt feed.Content) (interface{}, error) {
	// Get the related reactions.
	reactions := c.GetReactions(cnt.ContentID)
	// Get the related comments.
	comments := c.GetComments(cnt.ContentID)
	// If for any reason, we are not able to download it, return an error and an incomplete post.
	postBytes, err := c.Peer.DownloadPost(cnt.ContentID)
	if err != nil {
		return NewText("###", cnt, reactions, comments), err
	}
	if postBytes == nil {
		return NewText("###", cnt, reactions, comments), fmt.Errorf("could not download the post at client.downloadText")
	}
	// Otherwise, create the full post.
	text := content.ParseTextPost(postBytes)
	return NewText(text.Text, cnt, reactions, comments), nil
}

// In case of an error, returns an incomplete Comment and an error.
func (c *Client) downloadComment(cnt feed.Content) (interface{}, error) {
	// Get the related reactions.
	reactions := c.GetReactions(cnt.ContentID)
	// If for any reason, we are not able to download it, return an error and an incomplete post.
	postBytes, err := c.Peer.DownloadPost(cnt.ContentID)
	if err != nil {
		return NewComment("###", cnt, reactions), err
	}
	if postBytes == nil {
		return NewComment("###", cnt, reactions), fmt.Errorf("could not download the comment at client.downloadText")
	}
	// Otherwise, create the full post.
	text := content.ParseCommentPost(postBytes)
	return NewComment(text.Text, cnt, reactions), nil
}

// getDownloadableThings first loads the feed content from the feed store according to the given filter.
// Then, using the downloader, it tries to download the actual content.
// In case of download failure, it invokes the discovery before continuing.
func (c *Client) getDownloadableThings(filter content.Filter, downloader func(feed.Content) (interface{}, error)) []interface{} {
	// First, query the text content from the feed store.
	contents := c.Peer.QueryContents(filter)
	var thingsList []interface{}
	// Now, let's download the thing. Also check if we need to perform a discovery over the network.
	shouldDiscover := false
	lastContentIndex := -1
	for i, cnt := range contents {
		thing, err := downloader(cnt)
		if err != nil {
			lastContentIndex = i
			shouldDiscover = true
			break
		}
		thingsList = append(thingsList, thing)
	}
	// If required, discover the content and continue from where we left.
	if shouldDiscover {
		_, _ = c.Peer.DiscoverContentIDs(filter)
		for _, cnt := range contents[lastContentIndex:] {
			thing, _ := downloader(cnt)
			// If we still get an error, we simply append the incomplete thing. We don't care anymore x(
			thingsList = append(thingsList, thing)
		}
	}
	return thingsList
}
