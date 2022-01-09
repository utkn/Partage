package feed

type FeedContent struct {
	PostInfo
	Downloaded bool
}

func LoadFeedContentFromPostInfo(info PostInfo, autoDownload bool) FeedContent {
	return FeedContent{
		PostInfo:   info,
		Downloaded: false,
	}
}
