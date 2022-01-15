package impl

import (
	"encoding/hex"
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
	//GET & POST
	mux.Handle("/block", client.BlockHandler())
	// POST
	mux.Handle("/changeusername", client.ChangeUsernameHandler())

	err = http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	if err != nil {
		fmt.Println(err)
	}
}

func (c Client) ChangeUsernameHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// Add comment to post
			newUsername := r.FormValue("NewUsername")
			from := r.FormValue("from")
			//parse recipients list
			if newUsername != "" {
				_, err := c.Peer.UpdateFeed(content.CreateChangeUsernameMetadata(c.Peer.GetUserID(), newUsername))
				if err != nil {
					from = URLWithErrorMsg(from, err.Error())
				}
			} else {
				http.Error(w, "invalid", http.StatusNotAcceptable)
				return
			}
			http.Redirect(w, r, from, http.StatusSeeOther)
			return
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

//lacking filter implementation..TODO:!

//--------------------------
// Homepage handler
type Homepage struct {
	ErrorMsg string
	Username string
	UserID   template.HTML
	MyData   UserData
	Posts    []Text
}

var MaxTimeLimit = int64(0) //TODO: change..limit max time!
func (c Client) IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userdata := c.GetUserData(c.Peer.GetUserID())
			//<form action="/post" method="POST"> //TODO:
			p := Homepage{
				ErrorMsg: ParseErrorMsg(r),
				// Get userID
				UserID: template.HTML(c.Peer.GetUserID()),
				// Get username
				Username: userdata.Username,
				// Get Texts from Followes
				Posts:  c.GetTexts(userdata.Followees, 0, MaxTimeLimit),
				MyData: userdata,
			}
			t, err := template.ParseFiles(TemplateFileMap["base"], TemplateFileMap["index"], TemplatePath("components.html"))
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
	ErrorMsg string
	UserID   template.HTML
	Post     Text

	MyData UserData
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
			t, err := template.ParseFiles(TemplateFileMap["base"], TemplateFileMap["post"], TemplatePath("components.html"))
			if err != nil {
				fmt.Println(err)
				return
			}

			p := PostPage{
				ErrorMsg: ParseErrorMsg(r),
				Post:     *post,
				UserID:   template.HTML(c.Peer.GetUserID()),
				MyData:   c.GetUserData(c.Peer.GetUserID()),
			}
			t.Execute(w, p)

		case http.MethodPost:
			// Publish post
			content := r.FormValue("Content")
			from := r.FormValue("from")
			if content != "" {
				err := c.PostText(content)
				if err != nil {
					from = URLWithErrorMsg(from, err.Error())
				}
			}
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
			from := r.FormValue("from")
			if text != "" && len(recipientsArr) > 0 {
				err := c.PostPrivateText(text, recipientsArr)
				if err != nil {
					from = URLWithErrorMsg(from, err.Error())
				}
			} else {
				http.Error(w, "invalid", http.StatusNotAcceptable)
			}
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
			from := r.FormValue("from")
			if text != "" {
				err := c.PostComment(text, postID)
				if err != nil {
					from = URLWithErrorMsg(from, err.Error())
				}
			} else {
				http.Error(w, "invalid", http.StatusNotAcceptable)
			}
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
			from := r.FormValue("from")
			err := c.ReactToPost(reaction, postID)
			if err != nil {
				from = URLWithErrorMsg(from, err.Error())
			}
			http.Redirect(w, r, from, http.StatusSeeOther)
		case http.MethodGet:
			// Undo react made to post
			postID := r.FormValue("PostID")
			from := r.FormValue("from")
			err := c.UndoReaction(postID)
			if err != nil {
				from = URLWithErrorMsg(from, err.Error())
			}
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
	ErrorMsg      string
	Data          UserData
	FolloweeUsers []UserData
	FollowerUsers []UserData
	Posts         []Text
	IsMe          bool
	ImFollowedBy  bool //this user follows me
	IFollow       bool //i follow this user
	IsBlocked     bool

	// For navbar.
	UserID string
	// For the page itself.
	MyUserID string
	MyData   UserData
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
			var imFollowedBy, iFollow, isBlocked bool
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
				isBlocked = c.Peer.IsBlocked(UserID)
			}
			// Get the data of followers and followees.
			var followerUsers []UserData
			var followeeUsers []UserData
			for _, userID := range data.Followers {
				followerUsers = append(followerUsers, NewUserData(c.Peer.GetUserID(), c.Peer.GetUserState(userID)))
			}
			for _, userID := range data.Followees {
				followeeUsers = append(followeeUsers, NewUserData(c.Peer.GetUserID(), c.Peer.GetUserState(userID)))
			}

			profile := ProfilePage{
				ErrorMsg:      ParseErrorMsg(r),
				UserID:        c.Peer.GetUserID(),
				MyUserID:      c.Peer.GetUserID(),
				FollowerUsers: followerUsers,
				FolloweeUsers: followeeUsers,
				Data:          data,
				Posts:         texts,
				IsMe:          isMyProfile,
				ImFollowedBy:  imFollowedBy,
				IFollow:       iFollow,
				MyData:        c.GetUserData(c.Peer.GetUserID()),
				IsBlocked:     isBlocked,
			}
			// Render
			t, err := template.ParseFiles(TemplateFileMap["base"], TemplateFileMap["profile"], TemplatePath("components.html"))
			if err != nil {
				fmt.Println(err)
				return
			}
			t.Execute(w, profile)
		case http.MethodPost:
			// Follow
			userID := r.FormValue("UserID")
			from := r.FormValue("from")
			if userID != "" {
				err := c.FollowUser(userID)
				if err != nil {
					from = URLWithErrorMsg(from, err.Error())
				}
			}
			http.Redirect(w, r, from, http.StatusSeeOther)
		case http.MethodPut:
			// Unfollow
			userID := r.FormValue("UserID")
			from := r.FormValue("from")
			if userID != "" {
				err := c.UnfollowUser(userID)
				if err != nil {
					from = URLWithErrorMsg(from, err.Error())
				}
			}
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
			from := r.FormValue("from")
			if userID != "" {
				err := c.UnfollowUser(userID)
				if err != nil {
					from = URLWithErrorMsg(from, err.Error())
				}
			}
			http.Redirect(w, r, from, http.StatusSeeOther)
		case http.MethodGet:
			// Follow
			userID := r.FormValue("UserID")
			from := r.FormValue("from")
			if userID != "" {
				err := c.FollowUser(userID)
				if err != nil {
					from = URLWithErrorMsg(from, err.Error())
				}
			}
			http.Redirect(w, r, from, http.StatusSeeOther)
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

func (c Client) BlockHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// Block
			userID := r.FormValue("UserID")
			from := r.FormValue("from")
			if userID != "" {
				hashedPK, err := hex.DecodeString(userID)
				if err != nil {
					http.Redirect(w, r, from, http.StatusSeeOther)
					return
				}
				var hashedPKArray [32]byte
				copy(hashedPKArray[:], hashedPK)
				c.Peer.BlockUser([32]byte(hashedPKArray))
			}
			http.Redirect(w, r, from, http.StatusSeeOther)
		case http.MethodGet:
			// Unblock
			userID := r.FormValue("UserID")
			from := r.FormValue("from")
			if userID != "" {
				hashedPK, err := hex.DecodeString(userID)
				if err != nil {
					http.Redirect(w, r, from, http.StatusSeeOther)
					return
				}
				var hashedPKArray [32]byte
				copy(hashedPKArray[:], hashedPK)
				c.Peer.UnblockUser([32]byte(hashedPKArray))
			}
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
	ErrorMsg       string
	Posts          []Text
	SuggestedUsers []UserData

	UserID string
	MyData UserData
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
			// Convert to undiscovered user data.
			var undiscoveredUserData []UserData
			for _, uID := range undiscoveredUsers {
				undiscoveredUserData = append(undiscoveredUserData, NewUserData(c.Peer.GetUserID(), c.Peer.GetUserState(uID)))
			}
			var suggestedUsers []UserData
			if len(undiscoveredUsers) > 5 {
				// Append first 5 profiles to sugest
				suggestedUsers = append(suggestedUsers, undiscoveredUserData[:5]...)
			} else {
				// Append at most 3 profiles to sugest
				suggestedUsers = append(suggestedUsers, undiscoveredUserData...)
			}

			discoverPage := DiscoverPage{
				ErrorMsg:       ParseErrorMsg(r),
				UserID:         c.Peer.GetUserID(),
				Posts:          texts,
				SuggestedUsers: suggestedUsers,
				MyData:         c.GetUserData(c.Peer.GetUserID()),
			}
			// Render
			t, err := template.ParseFiles(TemplateFileMap["base"], TemplateFileMap["discover"], TemplatePath("components.html"))
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
			from := r.FormValue("from")
			// Request Endorsement
			err := c.RequestEndorsement()
			if err != nil {
				from = URLWithErrorMsg(from, err.Error())
			}
			http.Redirect(w, r, from, http.StatusSeeOther)
			return
		case http.MethodPost:
			// Endorse User
			userID := r.FormValue("UserID")
			from := r.FormValue("from")
			if userID != "" {
				err := c.EndorseUser(userID)
				if err != nil {
					from = URLWithErrorMsg(from, err.Error())
				}
			}
			http.Redirect(w, r, from, http.StatusSeeOther)
			return
		default:
			http.Error(w, "forbidden method", http.StatusMethodNotAllowed)
			return
		}
	}
}

func ParseErrorMsg(r *http.Request) string {
	errorMsg := ""
	if len(r.URL.Query()["ErrorMsg"]) > 0 {
		errorMsg = r.URL.Query()["ErrorMsg"][0]
	}
	return errorMsg
}

func URLWithErrorMsg(originalUrl string, errorMsg string) string {
	if strings.Contains(originalUrl, "?") {
		return originalUrl + "&ErrorMsg=" + errorMsg
	}
	return originalUrl + "?ErrorMsg=" + errorMsg
}
