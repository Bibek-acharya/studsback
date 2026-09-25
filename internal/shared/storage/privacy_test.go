package storage

import "testing"

func TestIsPrivateKey(t *testing.T) {
	cases := map[string]bool{
		"study-resources/lecture.mp4":         true,
		"study-resources/a/b/c.pdf":           true,
		"private/study-resources/lecture.mp4": true,
		"private/anything.txt":                true,
		"/uploads/private/x":                  true,
		"/study-resources/x.mp4":              true,
		"scholarship/../private/y":            true,
		"private/../private/y":                true,
		"../private/x.mp4":                    true,
		"study-resources-legacy/x.pdf":        false,
		"privately/x.pdf":                     false,
		"scholarship/documents/x.pdf":         false,
		"":                                    false,
		"publications/x.pdf":                  false,
		"uploads/scholarship/x.pdf":           false,
		"uploads/study-resources/a.pdf":       true,
	}
	for key, want := range cases {
		if got := IsPrivateKey(key); got != want {
			t.Errorf("IsPrivateKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestPrivatePrefixesCoverStudyResourcesAndVideos(t *testing.T) {
	if !IsPrivateKey(StudyResourcePrefix + "a.pdf") {
		t.Error("the study-resources prefix must be private")
	}
	if !IsPrivateKey(PrivateVideoPrefix + "a.mp4") {
		t.Error("the private video prefix must be private")
	}
}
