package impl

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"go.dedis.ch/cs438/registry/standard"
	"go.dedis.ch/cs438/storage/inmemory"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/transport/tcptls"
)

func TemplatePath(fileName string) string {
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage")]
	return rt + "Partage/Partage-Client/templates/" + fileName
}

var StaticFilePath = TemplatePath("") + "/static"

var TemplateFileMap = map[string]string{
	"index":    TemplatePath("index.html"),
	"post":     TemplatePath("post.html"),
	"profile":  TemplatePath("profile.html"),
	"discover": TemplatePath("discover.html"),
	"base":     TemplatePath("base.html"),
}

func NewDefaultConfig() peer.Configuration {
	return peer.Configuration{
		MessageRegistry:     standard.NewRegistry(),
		AntiEntropyInterval: time.Second,
		HeartbeatInterval:   2 * time.Second,
		AckTimeout:          time.Second * 3,
		ContinueMongering:   0.5,
		ChunkSize:           8192,
		// For now, we use an in-memory storage.
		Storage:           inmemory.NewPersistency(),
		BlockchainStorage: inmemory.NewPersistentMultipurposeStorage(),
		BackoffDataRequest: peer.Backoff{
			Initial: time.Second * 2,
			Factor:  2,
			Retry:   5,
		},
		TotalPeers: 1,
		PaxosThreshold: func(u uint) int {
			return int(u/2 + 1)
		},
		PaxosID:            1,
		PaxosProposerRetry: time.Second * 5,
	}
}

func StartClient(port uint, peerID uint, introducerAddr string) {
	mux := http.NewServeMux() //server multiplexer

	//create and initiate new Client instance.. TODO:
	nodeAddr := "127.0.0.1:0"
	transp := tcptls.NewTCP()
	// Create TLS socket
	sock, err := transp.CreateSocket(nodeAddr)
	if err != nil {
		fmt.Println("failed to create tls socket")
		return
	}
	// Create the configuration.
	config := NewDefaultConfig()
	config.Socket = sock
	config.PaxosID = peerID
	client := NewClient(1, introducerAddr, config)
	//Start node....

	// Start the static file server.
	fs := http.FileServer(http.Dir(StaticFilePath))
	// Start the actual server.
	// Serve the static file directory.
	mux.Handle("/static/", http.StripPrefix("/static", fs))
	// Homepage
	mux.Handle("/", client.IndexHandler())
	//GET & POST
	mux.Handle("/post", client.SinglePostHandler())
	//POST
	mux.Handle("/comment", client.CommentHandler())
	//POST & GET
	mux.Handle("/react", client.ReactHandler())
	//GET
	mux.Handle("/profile", client.ProfileHandler())
	//GET & POST
	mux.Handle("/user", client.UserHandler())
	//GET
	mux.Handle("/discover", client.DiscoverHandler())
	//GET & POST
	mux.Handle("/endorse", client.EndorsementHandler())
	//POST
	mux.Handle("/postPrivate", client.PrivatePostHandler())

	err = http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	if err != nil {
		fmt.Println(err)
	}
}

//lacking filter implementation..TODO:!

//--------------------------
// Homepage handler
type Homepage struct {
	Username        string
	UserID          template.HTML
	MyData          UserData
	Posts           []Text
	TimestampToDate func(int64) string
}

func timestampToDate(d int64) string {
	return time.Unix(d, 0).Format("15:04:05 2006-01-02 ")
}

var MaxTimeLimit = int64(0) //TODO: change..limit max time!
func (c Client) IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userdata := c.GetUserData(c.Peer.GetUserID())
			//<form action="/post" method="POST"> //TODO:
			p := Homepage{
				// Get userID
				UserID: template.HTML(c.Peer.GetUserID()),
				// Get username
				Username: userdata.Username,
				// Get Texts from Followes
				Posts:           c.GetTexts(c.GetUserData(c.Peer.GetUserID()).Followees, 0, MaxTimeLimit),
				TimestampToDate: timestampToDate,
				MyData:          userdata,
			}
			t, err := template.ParseFiles(TemplateFileMap["base"], TemplateFileMap["index"])
			if err != nil {
				fmt.Println(err)
				return
			}
			t.Execute(w, p)
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

//-------------------------
type PostPage struct {
	UserID          template.HTML
	Post            Text
	TimestampToDate func(int64) string
	MyData          UserData
}

// [GET] singular Post (all info) & [POST] create new post
func (c Client) SinglePostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			//localhost:8000/post/?PostID=........
			keys, ok := r.URL.Query()["PostID"]
			if !ok || len(keys[0]) < 1 {
				fmt.Println("Url Param 'PostID' is missing")
				return
			}
			// Query()["key"] will return an array of items, we only want the single item.
			PostID := keys[0]
			//PostID:=r.FormValue("PostID") //alternative
			posts := c.GetTexts(nil, 0, 0)
			var post *Text
			for _, txt := range posts {
				if txt.ContentID == PostID {
					post = &txt
					break
				}
			}
			if post == nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			// Render Post with all info, comments and reactions
			t, err := template.ParseFiles(TemplateFileMap["base"], TemplateFileMap["post"])
			if err != nil {
				fmt.Println(err)
				return
			}

			p := PostPage{
				TimestampToDate: timestampToDate,
				Post:            *post,
				UserID:          template.HTML(c.Peer.GetUserID()),
				MyData:          c.GetUserData(c.Peer.GetUserID()),
			}
			t.Execute(w, p)

		case http.MethodPost:
			// Publish post
			content := r.FormValue("Content")
			if content != "" {
				c.PostText(content)
			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
			return
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

//-------------------------
// [POST] share private Post
func (c Client) PrivatePostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// Add comment to post
			recipients := r.FormValue("Recipients")
			//parse recipients list
			recipientsArr := strings.Split(recipients, ",")
			text := r.FormValue("Content")
			if text != "" && len(recipientsArr) > 0 {
				err := c.PostPrivateText(text, recipientsArr)
				if err != nil {
					http.Error(w, "bad request", 400)
				}
			} else {
				http.Error(w, "invalid", http.StatusNotAcceptable)
			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

//-------------------------
// [POST] add comment to Post
func (c Client) CommentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// Add comment to post
			postID := r.FormValue("PostID")
			text := r.FormValue("Text")
			if text != "" {
				c.PostComment(text, postID)
			} else {
				http.Error(w, "invalid", http.StatusNotAcceptable)
			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

//-------------------------
// [POST] add react to Post & [DELETE] undos react made to post
func (c Client) ReactHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// Add react to post
			postID := r.FormValue("PostID")
			reactVal := r.FormValue("Reaction")
			reaction := stringToReaction(reactVal)
			if reaction.String() == "unknown" {
				http.Error(w, "invalid", http.StatusNotAcceptable)
				from := r.FormValue("from")
				http.Redirect(w, r, from, http.StatusSeeOther)
				return
			}
			fmt.Println(reaction)
			c.ReactToPost(reaction, postID)
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		case http.MethodGet:
			// Undo react made to post
			postID := r.FormValue("PostID")
			if err := c.UndoReaction(postID); err != nil {
				http.Error(w, "no content", http.StatusNoContent)

			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

func stringToReaction(r string) content.Reaction {
	switch r {
	case "happy":
		return content.HAPPY
	case "angry":
		return content.ANGRY
	case "confused":
		return content.CONFUSED
	case "approve":
		return content.APPROVE
	case "disapprove":
		return content.DISAPPROVE
	}
	return -1
}

//-------------------------
//Profile
type ProfilePage struct {
	MyData          UserData
	Posts           []Text
	IsMe            bool
	ImFollowedBy    bool //this user follows me
	IFollow         bool //i follow this user
	TimestampToDate func(int64) string
	// For navbar.
	UserID string
	// For the page itself.
	MyUserID string
}

// [GET] shows profile info and respective posts & [POST] is used to follow user & [PUT] is used to unfollow user
func (c Client) ProfileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			//localhost:8000/profile?PostID=........
			keys, ok := r.URL.Query()["UserID"]
			if !ok || len(keys[0]) < 1 {
				fmt.Println("Url Param 'UserID' is missing")
				http.Error(w, "parameter missing", http.StatusNotFound)
				return
			}
			UserID := keys[0]
			// Get user data
			data := c.GetUserData(UserID)
			// Get all posts from user
			texts := c.GetTexts([]string{UserID}, 0, 0)
			var imFollowedBy, iFollow bool
			isMyProfile := c.Peer.GetUserID() == UserID
			if !isMyProfile {
				for _, user := range data.Followers {
					if user == c.Peer.GetUserID() {
						iFollow = true
						break
					}
				}
				for _, user := range data.Followees {
					if user == c.Peer.GetUserID() {
						imFollowedBy = true
						break
					}
				}
			}
			profile := ProfilePage{
				UserID:          c.Peer.GetUserID(),
				MyUserID:        c.Peer.GetUserID(),
				TimestampToDate: timestampToDate,
				MyData:          data,
				Posts:           texts,
				IsMe:            isMyProfile,
				ImFollowedBy:    imFollowedBy,
				IFollow:         iFollow}
			// Render
			t, err := template.ParseFiles(TemplateFileMap["base"], TemplateFileMap["profile"])
			if err != nil {
				fmt.Println(err)
				return
			}
			t.Execute(w, profile)
		case http.MethodPost:
			// Follow
			userID := r.FormValue("UserID")
			if userID != "" {
				c.FollowUser(userID)
			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		case http.MethodPut:
			// Unfollow
			userID := r.FormValue("UserID")
			if userID != "" {
				c.UnfollowUser(userID)
			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

func (c Client) UserHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// Unfollow
			userID := r.FormValue("UserID")
			if userID != "" {
				c.UnfollowUser(userID)
			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		case http.MethodGet:
			// Follow
			userID := r.FormValue("UserID")
			if userID != "" {
				c.FollowUser(userID)
			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

//-------------------------
// Discover
type DiscoverPage struct {
	Posts           []Text
	SuggestedUsers  []string
	TimestampToDate func(int64) string
	UserID          string
	MyData          UserData
}

// [GET] shows suggested profiles to follow and latest posts from different users (users that are not followed by the user itself)
func (c Client) DiscoverHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			//localhost:8000/discover/
			knownUsers := c.Peer.GetKnownUsers()
			following := c.GetUserData(c.Peer.GetUserID()).Followees
			// Discard users that you're already following
			for _, user := range following {
				//if it exists in the knownUsers map..remove it
				delete(knownUsers, user)
			}
			delete(knownUsers, c.Peer.GetUserID())
			// slice with users that i'm not currently following
			undiscoveredUsers := make([]string, len(knownUsers))
			i := 0
			for user := range knownUsers {
				undiscoveredUsers[i] = user
				i++
			}

			// Get posts from users that i'm not currently following
			texts := c.GetTexts(undiscoveredUsers, 0, 0)

			var suggestedUsers []string
			if len(undiscoveredUsers) > 5 {
				// Append first 5 profiles to sugest
				suggestedUsers = append(suggestedUsers, undiscoveredUsers[:5]...)
			} else {
				// Append at most 3 profiles to sugest
				suggestedUsers = append(suggestedUsers, undiscoveredUsers...)
			}

			discoverPage := DiscoverPage{
				UserID:          c.Peer.GetUserID(),
				TimestampToDate: timestampToDate,
				Posts:           texts,
				SuggestedUsers:  suggestedUsers,
				MyData:          c.GetUserData(c.Peer.GetUserID()),
			}
			// Render
			t, err := template.ParseFiles(TemplateFileMap["base"], TemplateFileMap["discover"])
			if err != nil {
				fmt.Println(err)
				return
			}
			t.Execute(w, discoverPage)
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

func (c Client) EndorsementHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Request Endorsement
			err := c.RequestEndorsement()
			if err != nil {
				http.Error(w, "invalid", http.StatusNotAcceptable)
			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		case http.MethodPost:
			// Endorse User
			userID := r.FormValue("UserID")
			if userID != "" {
				err := c.EndorseUser(userID)
				if err != nil {
					http.Error(w, "invalid", http.StatusNotAcceptable)
				}
			}
			from := r.FormValue("from")
			http.Redirect(w, r, from, http.StatusSeeOther)
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}
