package feed

import (
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
)

// IsValidMetadata checks whether the given metadata can be safely added to the blockchain. Used during consensus.
func (feedStore *Store) IsValidMetadata(c content.Metadata) bool {
	// The user must be registered!
	if !feedStore.IsKnown(c.FeedUserID) {
		return false
	}
	// Accept reactions only when the user has not reacted to the referred content id yet.
	if c.Type == content.REACTION {
		return !feedStore.reactionHandler.AlreadyReacted(c.RefContentID, c.FeedUserID)
	}
	proposerFeed := feedStore.GetFeedCopy(c.FeedUserID)
	// Make sure that the user can afford the cost.
	if c.Type.Cost() > proposerFeed.userState.CurrentCredits {
		return false
	}
	// Only process an endorsement request if the user's credit is lower than the set amount.
	if c.Type == content.ENDORSEMENT_REQUEST {
		withinRange := proposerFeed.userState.CurrentCredits <= ENDORSEMENT_REQUEST_CREDIT_LIMIT
		return withinRange && proposerFeed.userState.EndorsementHandler.CanRequest()
	}
	// Reject follows for unknown users and double-follows.
	if c.Type == content.FOLLOW {
		targetUserID, _ := content.ParseFollowedUser(c)
		return feedStore.IsKnown(targetUserID) && !proposerFeed.userState.IsFollowing(targetUserID)
	}
	// Check whether the given endorsement is valid.
	if c.Type == content.ENDORSEMENT {
		referredUser, _ := content.ParseEndorsedUserID(c)
		referredFeed := feedStore.GetFeedCopy(referredUser)
		return feedStore.IsKnown(referredUser) && referredFeed.userState.CanEndorse(utils.Time(), c.FeedUserID)
	}
	// Check whether the attempted undo is valid.
	if c.Type == content.UNDO {
		referredHash, _ := content.ParseUndoMetadata(c)
		referredMetadata, err := proposerFeed.GetWithHash(referredHash)
		// The referred hash must exist.
		if err != nil {
			return false
		}
		// Referred metadata must be owned by the same user
		if c.FeedUserID != referredMetadata.FeedUserID {
			return false
		}
		// Only reactions, text, comments, and follows can be undone.
		referredMetadataIsUndoable :=
			referredMetadata.Type == content.REACTION ||
				referredMetadata.Type == content.TEXT ||
				referredMetadata.Type == content.COMMENT ||
				referredMetadata.Type == content.FOLLOW
		return referredMetadataIsUndoable
	}
	return true
}
