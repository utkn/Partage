package feed

type FeedContent struct {
	ContentMetadata
	Downloaded bool
}

func LoadFeedContentFromPostInfo(info ContentMetadata, autoDownload bool) FeedContent {
	return FeedContent{
		ContentMetadata: info,
		Downloaded:      false,
	}
}
