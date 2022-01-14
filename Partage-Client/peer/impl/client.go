package impl

import (
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/utils"
	"sort"
	"time"
)

// Client is a useful Partage Client to be used by a frontend.
type Client struct {
	Peer peer.SocialPeer
}

func NewClient(totalPeers uint, joinNodeAddr string, config peer.Configuration) *Client {
	fmt.Println("Starting client...")
	// TODO: calculate dynamically from the registration blockchain.
	config.TotalPeers = totalPeers
	// Create the peer.
	fmt.Println("Constructing peer...")
	p := NewPeer(config)
	// Add to the network.
	if joinNodeAddr != "" {
		p.AddPeer(joinNodeAddr)
	}
	// Try to start the node.
	err := p.Start()
	if err != nil {
		fmt.Printf("error during start: %v\n", err)
		return nil
	}
	// Load the registered users during the construction.
	fmt.Println("Loading registered users...")
	registered := p.(*node).social.LoadRegisteredUsers(config.BlockchainStorage)
	fmt.Println("Registered", registered, "many users.")
	// Initiate the self-register after acquiring the registration blockchain from the community.
	time.Sleep(5 * time.Second)
	fmt.Println("Initiating self-register...")
	err = p.RegisterUser()
	if err != nil {
		fmt.Printf("error during registration: %v\n", err)
		return nil
	}
	fmt.Println("OK!")
	// Return the client.
	return &Client{
		Peer: p,
	}
}

// GetUserData returns the user data associated with the given user id.
func (c *Client) GetUserData(userID string) UserData {
	selfID := c.Peer.GetUserID()
	return NewUserData(selfID, c.Peer.GetUserState(userID))
}

// GetTexts returns the texts with the given filters.
func (c *Client) GetTexts(userIDs []string, minTime int64, maxTime int64) []Text {
	utils.PrintDebug("social", "Client at GetTexts")
	// First, create the filter accordingly.
	filter := content.Filter{
		MaxTime:  maxTime,
		MinTime:  minTime,
		OwnerIDs: userIDs,
		Types:    []content.Type{content.TEXT},
	}
	textThings := c.getDownloadableThings(filter, c.downloadText)
	utils.PrintDebug("social", "Client fetched", len(textThings), "many texts")
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
	// Create an unencrypted content.
	cnt := content.NewPublicContent(c.Peer.GetUserID(), text, utils.Time(), "").Unencrypted()
	_, _, err := c.Peer.ShareDownloadableContent(cnt, content.TEXT)
	return err
}

func (c *Client) PostPrivateText(text string, recipientUserIDs []string) error {
	// Create the content.
	cnt := content.NewPublicContent(c.Peer.GetUserID(), text, utils.Time(), "")
	recipientMap, err := c.recipientListToRecipientMap(recipientUserIDs)
	if err != nil {
		return fmt.Errorf("error while parsing recipients: %v", err)
	}
	// Encrypt it.
	prCnt, err := cnt.Encrypted(recipientMap)
	if err != nil {
		return fmt.Errorf("error during encrypting private text: %v", err)
	}
	// Share the encrypted content.
	_, _, err = c.Peer.ShareDownloadableContent(prCnt, content.TEXT)
	return err
}

// PostComment posts a new comment. If the given post is private, the comment will also be encrypted in the same fashion.
func (c *Client) PostComment(comment string, postContentID string) error {
	// Download the content associated with the content id. Since we are posting a comment to it, we most likely have it in the local storage already.
	contents := c.getDownloadableThings(content.Filter{ContentID: postContentID}, c.downloadUploadedContent)
	if len(contents) != 1 {
		return fmt.Errorf("there are %d != 1 associated texts", len(contents))
	}
	// Get the recipients associated with this
	referredPostRecipientList := contents[0].(content.PrivateContent).RecipientList
	publicContent := content.NewPublicContent(c.Peer.GetUserID(), comment, utils.Time(), postContentID)
	var privateContent content.PrivateContent
	// Check the encryption status of the parent text post.
	if len(referredPostRecipientList) == 0 {
		// If it was not encrypted, do not encrypt the comment.
		privateContent = publicContent.Unencrypted()
	} else {
		// Directly use the parent post's recipient list.
		recptMap, err := c.recipientListToRecipientMap(referredPostRecipientList)
		if err != nil {
			return fmt.Errorf("could not encrypt comment: %v", err)
		}
		privateContent, err = publicContent.Encrypted(recptMap)
		if err != nil {
			return fmt.Errorf("could not encrypt comment: %v", err)
		}
	}
	// Post the comment. Finally.
	_, _, err := c.Peer.ShareDownloadableContent(privateContent, content.COMMENT)
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
	reactions := c.Peer.QueryFeedContents(content.Filter{
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
	follows := c.Peer.QueryFeedContents(content.Filter{
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
	downloadedBytes, err := c.Peer.DownloadContent(cnt.ContentID)
	if err != nil {
		return NewText("[error: could not fetch]", cnt, reactions, comments), err
	}
	if downloadedBytes == nil {
		return NewText("[error: could not fetch]", cnt, reactions, comments), fmt.Errorf("could not download the post at client.downloadText")
	}
	// Otherwise, create the full post.
	downloaded := content.ParseContent(downloadedBytes)
	// But first, try to decrypt.
	decrypted, err := downloaded.Decrypted(c.Peer.GetHashedPublicKey(), c.Peer.GetPrivateKey())
	return NewText(decrypted.Text, cnt, reactions, comments), nil
}

func (c *Client) downloadUploadedContent(cnt feed.Content) (interface{}, error) {
	// If for any reason, we are not able to download it, return an error and an incomplete post.
	downloadedBytes, err := c.Peer.DownloadContent(cnt.ContentID)
	if err != nil {
		return nil, err
	}
	if downloadedBytes == nil {
		return nil, fmt.Errorf("could not download the decryption data")
	}
	downloaded := content.ParseContent(downloadedBytes)
	return downloaded, nil
}

// In case of an error, returns an incomplete Comment and an error.
func (c *Client) downloadComment(cnt feed.Content) (interface{}, error) {
	// Get the related reactions.
	reactions := c.GetReactions(cnt.ContentID)
	// If for any reason, we are not able to download it, return an error and an incomplete post.
	downloadedBytes, err := c.Peer.DownloadContent(cnt.ContentID)
	if err != nil {
		return NewComment("[error: could not fetch]", cnt, reactions), err
	}
	if downloadedBytes == nil {
		return NewComment("[error: could not fetch]", cnt, reactions), fmt.Errorf("could not download the post at client.downloadText")
	}
	// Otherwise, create the full post.
	downloaded := content.ParseContent(downloadedBytes)
	// But first, try to decrypt.
	decrypted, err := downloaded.Decrypted(c.Peer.GetHashedPublicKey(), c.Peer.GetPrivateKey())
	return NewComment(decrypted.Text, cnt, reactions), nil
}

// getDownloadableThings first loads the feed content from the feed store according to the given filter.
// Then, using the downloader, it tries to download the actual content.
// In case of download failure, it invokes the discovery before continuing.
func (c *Client) getDownloadableThings(filter content.Filter, downloader func(feed.Content) (interface{}, error)) []interface{} {
	// First, query the text content from the feed store.
	contents := c.Peer.QueryFeedContents(filter)
	utils.PrintDebug("social", "Feed store returned", len(contents), "many metadata.")
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
		utils.PrintDebug("social", "Client initiating discovery...")
		cIDs, _ := c.Peer.DiscoverContentIDs(filter)
		utils.PrintDebug("social", "Discovery returned", len(cIDs), "many content IDs.")
		for _, cnt := range contents[lastContentIndex:] {
			thing, _ := downloader(cnt)
			// If we still get an error, we simply append the incomplete thing. We don't care anymore x(
			thingsList = append(thingsList, thing)
		}
	}
	return thingsList
}

func (c *Client) recipientListToRecipientMap(l []string) (map[[32]byte]*rsa.PublicKey, error) {
	m := make(map[[32]byte]*rsa.PublicKey)
	for _, r := range l {
		hashedPK, err := hex.DecodeString(r)
		if err != nil {
			return nil, fmt.Errorf("error while posting private text: malformed recipient %s", r)
		}
		var hashedPKArray [32]byte
		copy(hashedPKArray[:], hashedPK)
		m[hashedPKArray] = c.Peer.GetPublicKey(hashedPKArray)
	}
	return m, nil
}
