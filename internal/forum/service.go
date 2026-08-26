package forum

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"studsphere/backend/internal/shared/utils"

	"studsphere/backend/internal/shared/storage"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetForumCommunities(currentUserID uint) ([]CommunityResponse, error) {
	communities, err := s.repo.GetAllCommunities()
	if err != nil {
		return nil, errors.New("failed to fetch communities")
	}

	memberMap := make(map[uint]bool)
	if currentUserID != 0 {
		memberships, err := s.repo.GetMembershipsByUserID(currentUserID)
		if err == nil {
			for _, m := range memberships {
				memberMap[m.CommunityID] = true
			}
		}
	}

	var responses []CommunityResponse
	for _, c := range communities {
		memberCount, _ := s.repo.GetMemberCount(c.ID)
		postCount, _ := s.repo.GetPostCount(c.ID)

		responses = append(responses, CommunityResponse{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Icon:        c.Icon,
			BgColor:     c.BgColor,
			MemberCount: int(memberCount),
			IsMember:    memberMap[c.ID],
			IsGeneral:   c.IsGeneral,
			PostCount:   int(postCount),
		})
	}

	return responses, nil
}

func (s *Service) CreateForumCommunity(req CreateCommunityRequest) (*CommunityResponse, error) {
	community := &ForumCommunity{
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		BgColor:     req.BgColor,
	}
	if err := s.repo.CreateCommunity(community); err != nil {
		return nil, errors.New("failed to create community")
	}
	return &CommunityResponse{
		ID:          community.ID,
		Name:        community.Name,
		Description: community.Description,
		Icon:        community.Icon,
		BgColor:     community.BgColor,
		MemberCount: 0,
		IsMember:    false,
		IsGeneral:   community.IsGeneral,
		PostCount:   0,
	}, nil
}

func (s *Service) UpdateForumCommunity(communityID uint, req UpdateCommunityRequest) (*CommunityResponse, error) {
	community, err := s.repo.GetCommunityByID(communityID)
	if err != nil {
		return nil, errors.New("community not found")
	}

	community.Name = req.Name
	community.Description = req.Description
	community.Icon = req.Icon
	community.BgColor = req.BgColor

	if err := s.repo.UpdateCommunity(community); err != nil {
		return nil, errors.New("failed to update community")
	}

	memberCount, _ := s.repo.GetMemberCount(communityID)
	postCount, _ := s.repo.GetPostCount(communityID)

	return &CommunityResponse{
		ID:          community.ID,
		Name:        community.Name,
		Description: community.Description,
		Icon:        community.Icon,
		BgColor:     community.BgColor,
		IsGeneral:   community.IsGeneral,
		MemberCount: int(memberCount),
		PostCount:   int(postCount),
	}, nil
}

func (s *Service) DeleteForumCommunity(communityID uint) error {
	community, err := s.repo.GetCommunityByID(communityID)
	if err != nil {
		return errors.New("community not found")
	}

	if community.IsGeneral {
		return errors.New("the General community cannot be deleted")
	}

	posts, err := s.repo.GetPostsByCommunityID(communityID)
	if err != nil {
		return errors.New("failed to fetch community posts")
	}

	for _, post := range posts {
		s.deletePostFile(post.ImageURL)
		s.deletePostFile(post.VideoURL)

		s.repo.DeleteCommentsByPostID(post.ID)
		s.repo.DeleteVotesByPostID(post.ID)
		s.repo.DeleteSavesByPostID(post.ID)
		s.repo.DeletePollVotesByPostID(post.ID)
		s.repo.HardDeletePost(post.ID)
	}

	if err := s.repo.DeleteMembersByCommunityID(communityID); err != nil {
		return errors.New("failed to delete community members")
	}

	if err := s.repo.DeleteCommunity(community); err != nil {
		return errors.New("failed to delete community")
	}

	return nil
}

func (s *Service) deletePostFile(url string) {
	if url == "" {
		return
	}
	filename := strings.TrimPrefix(url, "/uploads/")
	if filename == url || filename == "" {
		return
	}
	storage.DeleteObject(filename)
}

func (s *Service) JoinForumCommunity(communityID uint, userID uint) (*CommunityResponse, error) {
	community, err := s.repo.GetCommunityByID(communityID)
	if err != nil {
		return nil, errors.New("community not found")
	}

	existing, err := s.repo.FindMembership(communityID, userID)
	var isMember bool

	if err == nil {
		s.repo.DeleteMembership(existing)
		isMember = false
	} else {
		member := &ForumCommunityMember{
			CommunityID: communityID,
			UserID:      userID,
		}
		s.repo.CreateMembership(member)
		isMember = true
	}

	memberCount, _ := s.repo.GetMemberCount(communityID)
	postCount, _ := s.repo.GetPostCount(communityID)

	return &CommunityResponse{
		ID:          community.ID,
		Name:        community.Name,
		Icon:        community.Icon,
		BgColor:     community.BgColor,
		MemberCount: int(memberCount),
		IsMember:    isMember,
		IsGeneral:   community.IsGeneral,
		PostCount:   int(postCount),
	}, nil
}

func (s *Service) GetForumPosts(category, communityID string, currentUserID uint) ([]PostResponse, error) {
	posts, err := s.repo.GetAllPosts(category, communityID, currentUserID)
	if err != nil {
		return nil, errors.New("failed to fetch posts")
	}

	if currentUserID == 0 {
		general, err := s.repo.FindCommunityByName("General")
		if err == nil {
			filtered := []ForumPost{}
			for _, p := range posts {
				if p.CommunityID == general.ID {
					filtered = append(filtered, p)
				}
			}
			posts = filtered
		}
	}

	if currentUserID != 0 {
		votes, _ := s.repo.GetVotesByUserID(currentUserID)
		voteMap := make(map[uint]int)
		for _, v := range votes {
			voteMap[v.PostID] = v.Vote
		}

		saves, _ := s.repo.GetSavesByUserID(currentUserID)
		saveMap := make(map[uint]bool)
		for _, sv := range saves {
			saveMap[sv.PostID] = true
		}

		for i := range posts {
			vote := voteMap[posts[i].ID]
			posts[i].IsLiked = vote == 1
			posts[i].IsDisliked = vote == -1
			posts[i].IsSaved = saveMap[posts[i].ID]
		}
	}

	pollPostIDs := []uint{}
	for _, p := range posts {
		if p.IsPoll {
			pollPostIDs = append(pollPostIDs, p.ID)
		}
	}

	if len(pollPostIDs) > 0 {
		allPollVotes, _ := s.repo.GetPollVotesByPostIDs(pollPostIDs)

		pollMap := make(map[uint]map[int]int)
		pollTotalMap := make(map[uint]int)
		for _, v := range allPollVotes {
			if _, ok := pollMap[v.PostID]; !ok {
				pollMap[v.PostID] = make(map[int]int)
			}
			pollMap[v.PostID][v.OptionIdx]++
			pollTotalMap[v.PostID]++
		}

		myPollVoteMap := make(map[uint]int)
		hasVotedMap := make(map[uint]bool)
		if currentUserID != 0 {
			for _, pid := range pollPostIDs {
				v, err := s.repo.GetPollVoteByPostAndUser(pid, currentUserID)
				if err == nil {
					myPollVoteMap[v.PostID] = v.OptionIdx
					hasVotedMap[v.PostID] = true
				}
			}
		}

		for i := range posts {
			id := posts[i].ID
			if posts[i].IsPoll {
				posts[i].PollResults = pollMap[id]
				posts[i].TotalVotes = pollTotalMap[id]
				if hasVotedMap[id] {
					opt := myPollVoteMap[id]
					posts[i].VotedOption = &opt
				}
			}
		}
	}

	var responses []PostResponse
	for _, p := range posts {
		responses = append(responses, mapPostToResponse(p))
	}

	return responses, nil
}

func (s *Service) GetTrendingForumPosts() ([]TrendingPostResponse, error) {
	posts, err := s.repo.GetTrendingPosts(5)
	if err != nil {
		return nil, errors.New("failed to fetch trending posts")
	}

	var responses []TrendingPostResponse
	for _, p := range posts {
		responses = append(responses, TrendingPostResponse{
			ID:           p.ID,
			Title:        p.Title,
			Category:     p.Category,
			Upvotes:      p.Upvotes,
			CommentCount: p.CommentCount,
		})
	}

	return responses, nil
}

func (s *Service) GetForumPostComments(postID uint, limit, offset int, sort string) (map[string]interface{}, error) {
	totalCount, err := s.repo.GetCommentCount(postID)
	if err != nil {
		return nil, errors.New("failed to fetch comments")
	}

	topComments, err := s.repo.GetTopComments(postID, limit, offset, sort)
	if err != nil {
		return nil, errors.New("failed to fetch comments")
	}

	parentIDs := make([]uint, len(topComments))
	for i, c := range topComments {
		parentIDs[i] = c.ID
	}
	replyCounts, _ := s.repo.GetReplyCountsByParentIDs(parentIDs)

	for i := range topComments {
		topComments[i].ReplyCount = replyCounts[topComments[i].ID]
	}

	var commentResponses []CommentResponse
	for _, c := range topComments {
		commentResponses = append(commentResponses, mapCommentToResponse(c))
	}

	return map[string]interface{}{
		"comments":    commentResponses,
		"total_count": totalCount,
	}, nil
}

func (s *Service) CreateForumPost(req CreatePostRequest, userID uint) (*PostResponse, error) {
	if req.CommunityID == 0 {
		general, err := s.repo.FindCommunityByName("General")
		if err != nil {
			return nil, errors.New("General community not found")
		}
		req.CommunityID = general.ID
	}

	community, err := s.repo.GetCommunityByID(req.CommunityID)
	if err != nil {
		return nil, errors.New("community not found")
	}
	if !community.IsGeneral {
		_, err = s.repo.FindMembership(req.CommunityID, userID)
		if err != nil {
			return nil, errors.New("you must join this community before posting")
		}
	}

	pollOptionsJSON, _ := json.Marshal(req.PollOptions)

	post := &ForumPost{
		UserID:      userID,
		CommunityID: req.CommunityID,
		Category:    req.Category,
		Title:       req.Title,
		Content:     req.Content,
		ImageURL:    req.ImageURL,
		VideoURL:    req.VideoURL,
		PollOptions: string(pollOptionsJSON),
		IsPoll:      req.IsPoll,
	}

	if err := s.repo.CreatePost(post); err != nil {
		return nil, errors.New("failed to create post")
	}

	post, err = s.repo.GetPostByID(post.ID)
	if err != nil {
		return nil, errors.New("failed to fetch created post")
	}

	resp := mapPostToResponse(*post)
	return &resp, nil
}

func (s *Service) UpdateForumPost(postID uint, userID uint, req UpdatePostRequest) (*PostResponse, error) {
	post, err := s.repo.GetPostByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}

	if post.UserID != userID {
		return nil, errors.New("you can only edit your own posts")
	}

	if req.Title != nil {
		post.Title = *req.Title
	}
	if req.Content != nil {
		post.Content = *req.Content
	}

	if err := s.repo.UpdatePost(post); err != nil {
		return nil, errors.New("failed to update post")
	}

	post, err = s.repo.GetPostByID(post.ID)
	if err != nil {
		return nil, errors.New("failed to fetch updated post")
	}

	resp := mapPostToResponse(*post)
	return &resp, nil
}

func (s *Service) DeleteForumPost(postID uint, userID uint) error {
	post, err := s.repo.GetPostByID(postID)
	if err != nil {
		return errors.New("post not found")
	}

	if post.UserID != userID {
		return errors.New("you can only delete your own posts")
	}

	if err := s.repo.DeletePost(post); err != nil {
		return errors.New("failed to delete post")
	}

	return nil
}

func (s *Service) AdminDeleteForumPost(postID uint) error {
	post, err := s.repo.GetPostByID(postID)
	if err != nil {
		return errors.New("post not found")
	}

	s.deletePostFile(post.ImageURL)
	s.deletePostFile(post.VideoURL)

	s.repo.DeleteCommentsByPostID(post.ID)
	s.repo.DeleteVotesByPostID(post.ID)
	s.repo.DeleteSavesByPostID(post.ID)
	s.repo.DeletePollVotesByPostID(post.ID)

	if err := s.repo.HardDeletePost(post.ID); err != nil {
		return errors.New("failed to delete post")
	}

	return nil
}

func (s *Service) LikeForumPost(postID uint, userID uint) (*PostResponse, error) {
	post, err := s.repo.GetPostByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}

	existingVote, err := s.repo.FindVote(postID, userID)

	if err == nil {
		if existingVote.Vote == 1 {
			s.repo.DeleteVote(existingVote)
			if post.Upvotes > 0 {
				post.Upvotes--
			}
			post.IsLiked = false
		} else {
			existingVote.Vote = 1
			s.repo.UpdateVote(existingVote)
			post.Upvotes++
			if post.Downvotes > 0 {
				post.Downvotes--
			}
			post.IsLiked = true
			post.IsDisliked = false
		}
	} else {
		vote := &ForumVote{
			PostID: postID,
			UserID: userID,
			Vote:   1,
		}
		s.repo.CreateVote(vote)
		post.Upvotes++
		post.IsLiked = true
	}

	s.repo.UpdatePostVotes(postID, post.Upvotes, post.Downvotes)

	resp := mapPostToResponse(*post)
	return &resp, nil
}

func (s *Service) DislikeForumPost(postID uint, userID uint) (*PostResponse, error) {
	post, err := s.repo.GetPostByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}

	existingVote, err := s.repo.FindVote(postID, userID)

	if err == nil {
		if existingVote.Vote == -1 {
			s.repo.DeleteVote(existingVote)
			if post.Downvotes > 0 {
				post.Downvotes--
			}
			post.IsDisliked = false
		} else {
			existingVote.Vote = -1
			s.repo.UpdateVote(existingVote)
			post.Downvotes++
			if post.Upvotes > 0 {
				post.Upvotes--
			}
			post.IsLiked = false
			post.IsDisliked = true
		}
	} else {
		vote := &ForumVote{
			PostID: postID,
			UserID: userID,
			Vote:   -1,
		}
		s.repo.CreateVote(vote)
		post.Downvotes++
		post.IsDisliked = true
	}

	s.repo.UpdatePostVotes(postID, post.Upvotes, post.Downvotes)

	resp := mapPostToResponse(*post)
	return &resp, nil
}

func (s *Service) SaveForumPost(postID uint, userID uint) (*PostResponse, error) {
	post, err := s.repo.GetPostByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}

	_, err = s.repo.FindSave(postID, userID)

	if err == nil {
		existingSave, _ := s.repo.FindSave(postID, userID)
		s.repo.DeleteSave(existingSave)
		post.IsSaved = false
	} else {
		save := &ForumSave{
			PostID: postID,
			UserID: userID,
		}
		s.repo.CreateSave(save)
		post.IsSaved = true
	}

	resp := mapPostToResponse(*post)
	return &resp, nil
}

func (s *Service) CreateForumComment(postID uint, userID uint, req CreateCommentRequest) (*CommentResponse, error) {
	comment := &ForumComment{
		PostID:   postID,
		UserID:   userID,
		Content:  req.Content,
		ImageURL: req.ImageURL,
		ParentID: req.ParentID,
	}

	if err := s.repo.CreateComment(comment); err != nil {
		return nil, errors.New("failed to create comment")
	}

	s.repo.IncrementCommentCount(postID)

	comment, err := s.repo.GetCommentByID(comment.ID)
	if err != nil {
		return nil, errors.New("failed to fetch created comment")
	}

	resp := mapCommentToResponse(*comment)
	return &resp, nil
}

func (s *Service) VoteForumPoll(postID uint, userID uint, optionIdx int) (*PostResponse, error) {
	post, err := s.repo.GetPostByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}

	if !post.IsPoll {
		return nil, errors.New("post is not a poll")
	}

	existingVote, err := s.repo.GetPollVoteByPostAndUser(postID, userID)
	if err == nil {
		existingVote.OptionIdx = optionIdx
		s.repo.UpdatePollVote(existingVote)
	} else {
		vote := &ForumPollVote{
			PostID:    postID,
			UserID:    userID,
			OptionIdx: optionIdx,
		}
		s.repo.CreatePollVote(vote)
	}

	allPollVotes, _ := s.repo.GetAllPollVotesByPostID(postID)

	results := make(map[int]int)
	total := 0
	for _, v := range allPollVotes {
		results[v.OptionIdx]++
		total++
	}

	post.PollResults = results
	post.TotalVotes = total
	post.VotedOption = &optionIdx

	resp := mapPostToResponse(*post)
	return &resp, nil
}

func (s *Service) UploadForumMedia(files []*multipart.FileHeader) ([]string, error) {
	if len(files) == 0 {
		return nil, errors.New("no files provided")
	}

	var urls []string
	for _, file := range files {
		ct := file.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "image/") && !strings.HasPrefix(ct, "video/") {
			continue
		}

		url, err := utils.SaveUploadedImage(file, "forum")
		if err != nil {
			return nil, fmt.Errorf("failed to save file: %s", file.Filename)
		}

		urls = append(urls, url)
	}

	if len(urls) == 0 {
		return nil, errors.New("no valid image/video files were uploaded")
	}

	return urls, nil
}

func mapPostToResponse(post ForumPost) PostResponse {
	resp := PostResponse{
		ID:           post.ID,
		CreatedAt:    post.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    post.UpdatedAt.Format(time.RFC3339),
		UserID:       post.UserID,
		CommunityID:  post.CommunityID,
		Category:     post.Category,
		Title:        post.Title,
		Content:      post.Content,
		ImageURL:     post.ImageURL,
		VideoURL:     post.VideoURL,
		Upvotes:      post.Upvotes,
		Downvotes:    post.Downvotes,
		CommentCount: post.CommentCount,
		IsPoll:       post.IsPoll,
		PollOptions:  post.PollOptions,
		IsLiked:      post.IsLiked,
		IsDisliked:   post.IsDisliked,
		IsSaved:      post.IsSaved,
		VotedOption:  post.VotedOption,
		PollResults:  post.PollResults,
		TotalVotes:   post.TotalVotes,
	}

	if post.User.ID != 0 {
		resp.UserName = post.User.FirstName + " " + post.User.LastName
		resp.User = &UserInfo{
			ID:        post.User.ID,
			FirstName: post.User.FirstName,
			LastName:  post.User.LastName,
			Email:     post.User.Email,
			ImageURL:  post.User.ImageURL,
		}
	}

	return resp
}

func mapCommentToResponse(comment ForumComment) CommentResponse {
	resp := CommentResponse{
		ID:         comment.ID,
		CreatedAt:  comment.CreatedAt.Format(time.RFC3339),
		PostID:     comment.PostID,
		UserID:     comment.UserID,
		Content:    comment.Content,
		ImageURL:   comment.ImageURL,
		ParentID:   comment.ParentID,
		ReplyCount: comment.ReplyCount,
	}

	if comment.User.ID != 0 {
		resp.User = CommentUser{
			FirstName: comment.User.FirstName,
			LastName:  comment.User.LastName,
		}
		resp.UserName = comment.User.FirstName + " " + comment.User.LastName
	}

	for _, r := range comment.Replies {
		resp.Replies = append(resp.Replies, mapCommentToResponse(r))
	}

	return resp
}

func (s *Service) ReportPost(postID, userID uint, req ReportPostRequest) error {
	report := &ForumReport{
		PostID:    postID,
		UserID:    userID,
		Reasons:   strings.Join(req.Reasons, ","),
		OtherText: req.OtherText,
	}
	return s.repo.CreateReport(report)
}

func (s *Service) NotInterested(postID, userID uint) error {
	ni := &ForumNotInterested{
		PostID: postID,
		UserID: userID,
	}
	return s.repo.CreateNotInterested(ni)
}

func (s *Service) GetNotInterestedPostIDs(userID uint) ([]uint, error) {
	return s.repo.GetNotInterestedPostIDs(userID)
}

func ParseUint(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}
