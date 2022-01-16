package feed

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
)

// CheckMetadata checks the validity of the given metadata. Returns either an error string explaining the issue or nil
// in case the metadata is valid.
func (feedStore *Store) CheckMetadata(c content.Metadata) error {
	// The user must be registered!
	if !feedStore.IsKnown(c.FeedUserID) {
		return fmt.Errorf("user is not registered")
	}
	// Accept reactions only when the user has not reacted to the referred content id yet.
	if c.Type == content.REACTION {
		alreadyReacted := feedStore.reactionHandler.AlreadyReacted(c.RefContentID, c.FeedUserID)
		if alreadyReacted {
			return fmt.Errorf("already reacted")
		}
	}
	proposerFeed := feedStore.GetFeedCopy(c.FeedUserID)
	// Make sure that the user can afford the cost.
	if c.Type.Cost() > proposerFeed.userState.CurrentCredits {
		return fmt.Errorf("not enough credits")
	}
	// Only process an endorsement request if the user's credit is lower than the set amount.
	if c.Type == content.ENDORSEMENT_REQUEST {
		withinRange := proposerFeed.userState.CurrentCredits <= ENDORSEMENT_REQUEST_CREDIT_LIMIT
		if !withinRange {
			return fmt.Errorf("too rich to request endorsements")
		}
		if !proposerFeed.userState.EndorsementHandler.CanRequest(c.Timestamp) {
			return fmt.Errorf("cannot request endorsements")
		}
	}
	// Reject follows for unknown users and double-follows.
	if c.Type == content.FOLLOW {
		targetUserID, _ := content.ParseFollowedUser(c)
		if !feedStore.IsKnown(targetUserID) {
			return fmt.Errorf("user to follow is not known")
		}
		if proposerFeed.userState.IsFollowing(targetUserID) {
			return fmt.Errorf("already following user")
		}
	}
	// Check whether the given endorsement is valid.
	if c.Type == content.ENDORSEMENT {
		referredUser, _ := content.ParseEndorsedUserID(c)
		referredFeed := feedStore.GetFeedCopy(referredUser)
		if !feedStore.IsKnown(referredUser) {
			return fmt.Errorf("user to endorse is not known")
		}
		if !referredFeed.userState.CanEndorse(utils.Time(), c.FeedUserID) {
			return fmt.Errorf("cannot endorse the user")
		}
	}
	// Check whether the attempted undo is valid.
	if c.Type == content.UNDO {
		referredHash, _ := content.ParseUndoMetadata(c)
		referredMetadata, err := proposerFeed.GetWithHash(referredHash)
		// The referred hash must exist.
		if err != nil {
			return fmt.Errorf("nothing to undo")
		}
		// Referred metadata must be owned by the same user
		if c.FeedUserID != referredMetadata.FeedUserID {
			return fmt.Errorf("cannot undo other people's stuff")
		}
		// Only reactions, text, comments, and follows can be undone.
		referredMetadataIsUndoable :=
			referredMetadata.Type == content.REACTION ||
				referredMetadata.Type == content.TEXT ||
				referredMetadata.Type == content.COMMENT ||
				referredMetadata.Type == content.FOLLOW
		if !referredMetadataIsUndoable {
			return fmt.Errorf("content is not undoable")
		}
	}
	return nil
}
