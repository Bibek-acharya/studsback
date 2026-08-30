package forum

import "testing"

func TestMapCommentToResponseIncludesUserImageURL(t *testing.T) {
	comment := ForumComment{
		UserID: 42,
		User: User{
			ID:        42,
			FirstName: "Ada",
			LastName:  "Lovelace",
			ImageURL:  "/uploads/avatars/ada.jpg",
		},
	}

	response := mapCommentToResponse(comment)

	if response.User.ImageURL != comment.User.ImageURL {
		t.Fatalf("expected comment user image URL %q, got %q", comment.User.ImageURL, response.User.ImageURL)
	}
}
