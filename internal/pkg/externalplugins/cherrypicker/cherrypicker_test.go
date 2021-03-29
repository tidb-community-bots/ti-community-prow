/*
Copyright 2017 The Kubernetes Authors.
Copyright 2021 The TiChi Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

The original file of the code is at:
https://github.com/kubernetes/test-infra/blob/master/prow/external-plugins/cherrypicker/server_test.go,
which we modified to add support for copying the labels and reviewers.
*/

package cherrypicker

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/ti-community-infra/tichi/internal/pkg/externalplugins"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/git/localgit"
	"k8s.io/test-infra/prow/github"
)

var commentFormat = "%s/%s#%d %s"

type fghc struct {
	sync.Mutex
	pr       *github.PullRequest
	isMember bool

	patch      []byte
	comments   []string
	prs        []github.PullRequest
	prComments []github.IssueComment
	prLabels   []github.Label
	orgMembers []github.TeamMember
	issues     []github.Issue
}

func (f *fghc) AddLabel(org, repo string, number int, label string) error {
	f.Lock()
	defer f.Unlock()
	for i := range f.prs {
		if number == f.prs[i].Number {
			f.prs[i].Labels = append(f.prs[i].Labels, github.Label{Name: label})
		}
	}
	return nil
}

func (f *fghc) AssignIssue(org, repo string, number int, logins []string) error {
	f.Lock()
	defer f.Unlock()
	var users []github.User
	for _, login := range logins {
		users = append(users, github.User{Login: login})
	}
	for i := range f.prs {
		if number == f.prs[i].Number {
			f.prs[i].Assignees = users
		}
	}
	return nil
}

func (f *fghc) RequestReview(org, repo string, number int, logins []string) error {
	f.Lock()
	defer f.Unlock()
	var users []github.User
	for _, login := range logins {
		users = append(users, github.User{Login: login})
	}
	for i := range f.prs {
		if number == f.prs[i].Number {
			f.prs[i].RequestedReviewers = users
		}
	}
	return nil
}

func (f *fghc) GetPullRequest(org, repo string, number int) (*github.PullRequest, error) {
	f.Lock()
	defer f.Unlock()
	return f.pr, nil
}

func (f *fghc) GetPullRequestPatch(org, repo string, number int) ([]byte, error) {
	f.Lock()
	defer f.Unlock()
	return f.patch, nil
}

func (f *fghc) GetPullRequests(org, repo string) ([]github.PullRequest, error) {
	f.Lock()
	defer f.Unlock()
	return f.prs, nil
}

func (f *fghc) CreateComment(org, repo string, number int, comment string) error {
	f.Lock()
	defer f.Unlock()
	f.comments = append(f.comments, fmt.Sprintf(commentFormat, org, repo, number, comment))
	return nil
}

func (f *fghc) IsMember(org, user string) (bool, error) {
	f.Lock()
	defer f.Unlock()
	return f.isMember, nil
}

func (f *fghc) GetRepo(owner, name string) (github.FullRepo, error) {
	f.Lock()
	defer f.Unlock()
	return github.FullRepo{}, nil
}

func (f *fghc) EnsureFork(forkingUser, org, repo string) (string, error) {
	if repo == "changeme" {
		return "changed", nil
	}
	if repo == "error" {
		return repo, errors.New("errors")
	}
	return repo, nil
}

var expectedFmt = `title=%q body=%q head=%s base=%s labels=%v reviewers=%v assignees=%v`

func prToString(pr github.PullRequest) string {
	var labels []string
	for _, label := range pr.Labels {
		labels = append(labels, label.Name)
	}

	var reviewers []string
	for _, reviewer := range pr.RequestedReviewers {
		reviewers = append(reviewers, reviewer.Login)
	}

	var assignees []string
	for _, assignee := range pr.Assignees {
		assignees = append(assignees, assignee.Login)
	}
	return fmt.Sprintf(expectedFmt, pr.Title, pr.Body, pr.Head.Ref, pr.Base.Ref, labels, reviewers, assignees)
}

func (f *fghc) CreateIssue(org, repo, title, body string, milestone int, labels, assignees []string) (int, error) {
	f.Lock()
	defer f.Unlock()

	var ghLabels []github.Label
	var ghAssignees []github.User

	num := len(f.issues) + 1

	for _, label := range labels {
		ghLabels = append(ghLabels, github.Label{Name: label})
	}

	for _, assignee := range assignees {
		ghAssignees = append(ghAssignees, github.User{Login: assignee})
	}

	f.issues = append(f.issues, github.Issue{
		Title:     title,
		Body:      body,
		Number:    num,
		Labels:    ghLabels,
		Assignees: ghAssignees,
	})

	return num, nil
}

func (f *fghc) CreatePullRequest(org, repo, title, body, head, base string, canModify bool) (int, error) {
	f.Lock()
	defer f.Unlock()
	num := len(f.prs) + 1
	f.prs = append(f.prs, github.PullRequest{
		Title:  title,
		Body:   body,
		Number: num,
		Head:   github.PullRequestBranch{Ref: head},
		Base:   github.PullRequestBranch{Ref: base},
	})
	return num, nil
}

func (f *fghc) ListIssueComments(org, repo string, number int) ([]github.IssueComment, error) {
	f.Lock()
	defer f.Unlock()
	return f.prComments, nil
}

func (f *fghc) GetIssueLabels(org, repo string, number int) ([]github.Label, error) {
	f.Lock()
	defer f.Unlock()
	return f.prLabels, nil
}

func (f *fghc) ListOrgMembers(org, role string) ([]github.TeamMember, error) {
	f.Lock()
	defer f.Unlock()
	if role != "all" {
		return nil, fmt.Errorf("all is only supported role, not: %s", role)
	}
	return f.orgMembers, nil
}

func (f *fghc) CreateFork(org, repo string) (string, error) {
	return repo, nil
}

var initialFiles = map[string][]byte{
	"bar.go": []byte(`// Package bar does an interesting thing.
package bar

// Foo does a thing.
func Foo(wow int) int {
	return 42 + wow
}
`),
}

var patch = []byte(`From af468c9e69dfdf39db591f1e3e8de5b64b0e62a2 Mon Sep 17 00:00:00 2001
From: Wise Guy <wise@guy.com>
Date: Thu, 19 Oct 2017 15:14:36 +0200
Subject: [PATCH] Update magic number

---
 bar.go | 3 ++-
 1 file changed, 2 insertions(+), 1 deletion(-)

diff --git a/bar.go b/bar.go
index 1ea52dc..5bd70a9 100644
--- a/bar.go
+++ b/bar.go
@@ -3,5 +3,6 @@ package bar

 // Foo does a thing.
 func Foo(wow int) int {
-	return 42 + wow
+	// Needs to be 49 because of a reason.
+	return 49 + wow
 }
`)

var body = "This PR updates the magic number.\n\n```release-note\nUpdate the magic number from 42 to 49\n```"

func TestCherryPickIC(t *testing.T) {
	t.Parallel()
	testCherryPickIC(localgit.New, t)
}

func TestCherryPickICV2(t *testing.T) {
	t.Parallel()
	testCherryPickIC(localgit.NewV2, t)
}

func testCherryPickIC(clients localgit.Clients, t *testing.T) {
	lg, c, err := clients()
	if err != nil {
		t.Fatalf("Making localgit: %v", err)
	}
	defer func() {
		if err := lg.Clean(); err != nil {
			t.Errorf("Cleaning up localgit: %v", err)
		}
		if err := c.Clean(); err != nil {
			t.Errorf("Cleaning up client: %v", err)
		}
	}()
	if err := lg.MakeFakeRepo("foo", "bar"); err != nil {
		t.Fatalf("Making fake repo: %v", err)
	}
	if err := lg.AddCommit("foo", "bar", initialFiles); err != nil {
		t.Fatalf("Adding initial commit: %v", err)
	}
	if err := lg.CheckoutNewBranch("foo", "bar", "stage"); err != nil {
		t.Fatalf("Checking out pull branch: %v", err)
	}

	ghc := &fghc{
		pr: &github.PullRequest{
			Base: github.PullRequestBranch{
				Ref: "master",
			},
			Number: 2,
			Merged: true,
			Title:  "This is a fix for X",
			Body:   body,
			RequestedReviewers: []github.User{
				{
					Login: "user1",
				},
			},
			Assignees: []github.User{
				{
					Login: "user2",
				},
			},
		},
		isMember: true,
		patch:    patch,
	}
	ic := github.IssueCommentEvent{
		Action: github.IssueCommentActionCreated,
		Repo: github.Repo{
			Owner: github.User{
				Login: "foo",
			},
			Name:     "bar",
			FullName: "foo/bar",
		},
		Issue: github.Issue{
			Number:      2,
			State:       "closed",
			PullRequest: &struct{}{},
		},
		Comment: github.IssueComment{
			User: github.User{
				Login: "wiseguy",
			},
			Body: "/cherrypick stage",
		},
	}

	botUser := &github.UserData{Login: "ci-robot", Email: "ci-robot@users.noreply.github.com"}
	expectedTitle := "This is a fix for X (#2)[stage]"
	expectedBody := "This is an automated cherry-pick of #2\n\nThis PR updates the magic number.\n\n```release-note\n" +
		"Update the magic number from 42 to 49\n```"
	expectedBase := "stage"
	expectedHead := fmt.Sprintf(botUser.Login+":"+cherryPickBranchFmt, 2, expectedBase)
	var expectedLabels []string
	expectedReviewers := []string{"user1"}
	expectedAssignees := []string{"wiseguy"}
	expected := fmt.Sprintf(expectedFmt, expectedTitle, expectedBody, expectedHead,
		expectedBase, expectedLabels, expectedReviewers, expectedAssignees)

	getSecret := func() []byte {
		return []byte("sha=abcdefg")
	}

	cfg := &externalplugins.Configuration{}
	cfg.TiCommunityCherrypicker = []externalplugins.TiCommunityCherrypicker{
		{
			Repos:       []string{"foo/bar"},
			LabelPrefix: "cherrypick/",
		},
	}
	ca := &externalplugins.ConfigAgent{}
	ca.Set(cfg)

	s := &Server{
		BotUser:        botUser,
		GitClient:      c,
		ConfigAgent:    ca,
		Push:           func(forkName, newBranch string, force bool) error { return nil },
		GitHubClient:   ghc,
		TokenGenerator: getSecret,
		Log:            logrus.StandardLogger().WithField("client", "cherrypicker"),
		Repos:          []github.Repo{{Fork: true, FullName: "ci-robot/bar"}},
	}

	if err := s.handleIssueComment(logrus.NewEntry(logrus.StandardLogger()), ic); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := prToString(ghc.prs[0])
	if got != expected {
		t.Errorf("Expected (%d):\n%s\nGot (%d):\n%+v\n", len(expected), expected, len(got), got)
	}
}

func TestCherryPickPR(t *testing.T) {
	t.Parallel()
	testCherryPickPR(localgit.New, t)
}

func TestCherryPickPRV2(t *testing.T) {
	t.Parallel()
	testCherryPickPR(localgit.NewV2, t)
}

func testCherryPickPR(clients localgit.Clients, t *testing.T) {
	lg, c, err := clients()
	if err != nil {
		t.Fatalf("Making localgit: %v", err)
	}
	defer func() {
		if err := lg.Clean(); err != nil {
			t.Errorf("Cleaning up localgit: %v", err)
		}
		if err := c.Clean(); err != nil {
			t.Errorf("Cleaning up client: %v", err)
		}
	}()
	if err := lg.MakeFakeRepo("foo", "bar"); err != nil {
		t.Fatalf("Making fake repo: %v", err)
	}
	if err := lg.AddCommit("foo", "bar", initialFiles); err != nil {
		t.Fatalf("Adding initial commit: %v", err)
	}
	if err := lg.CheckoutNewBranch("foo", "bar", "release-1.5"); err != nil {
		t.Fatalf("Checking out pull branch: %v", err)
	}
	if err := lg.CheckoutNewBranch("foo", "bar", "release-1.6"); err != nil {
		t.Fatalf("Checking out pull branch: %v", err)
	}
	if err := lg.CheckoutNewBranch("foo", "bar", "cherry-pick-2-to-release-1.5"); err != nil {
		t.Fatalf("Checking out existing PR branch: %v", err)
	}

	ghc := &fghc{
		orgMembers: []github.TeamMember{
			{
				Login: "approver",
			},
			{
				Login: "merge-bot",
			},
		},
		prComments: []github.IssueComment{
			{
				User: github.User{
					Login: "developer",
				},
				Body: "a review comment",
			},
			{
				User: github.User{
					Login: "approver",
				},
				Body: "/cherrypick release-1.5\r",
			},
			{
				User: github.User{
					Login: "approver",
				},
				Body: "/cherrypick release-1.6",
			},
			{
				User: github.User{
					Login: "fan",
				},
				Body: "/cherrypick release-1.7",
			},
			{
				User: github.User{
					Login: "approver",
				},
				Body: "/approve",
			},
			{
				User: github.User{
					Login: "merge-bot",
				},
				Body: "Automatic merge from submit-queue.",
			},
		},
		prs: []github.PullRequest{
			{
				Title: "This is a fix for Y (#2)[release-1.5]",
				Body:  "This is an automated cherry-pick of #2",
				Base: github.PullRequestBranch{
					Ref: "release-1.5",
				},
				Head: github.PullRequestBranch{
					Ref: "ci-robot:cherry-pick-2-to-release-1.5",
				},
				Labels: []github.Label{
					{
						Name: "test",
					},
				},
				RequestedReviewers: []github.User{
					{
						Login: "user1",
					},
				},
				Assignees: []github.User{
					{
						Login: "approver",
					},
				},
			},
		},
		isMember: true,
		patch:    patch,
	}
	pr := github.PullRequestEvent{
		Action: github.PullRequestActionClosed,
		PullRequest: github.PullRequest{
			Base: github.PullRequestBranch{
				Ref: "master",
				Repo: github.Repo{
					Owner: github.User{
						Login: "foo",
					},
					Name: "bar",
				},
			},
			Number:   2,
			Merged:   true,
			MergeSHA: new(string),
			Title:    "This is a fix for Y",
			Labels: []github.Label{
				{
					Name: "test",
				},
			},
			RequestedReviewers: []github.User{
				{
					Login: "user1",
				},
			},
			Assignees: []github.User{
				{
					Login: "approver",
				},
			},
		},
	}

	botUser := &github.UserData{Login: "ci-robot", Email: "ci-robot@users.noreply.github.com"}

	getSecret := func() []byte {
		return []byte("sha=abcdefg")
	}

	cfg := &externalplugins.Configuration{}
	cfg.TiCommunityCherrypicker = []externalplugins.TiCommunityCherrypicker{
		{
			Repos:       []string{"foo/bar"},
			LabelPrefix: "cherrypick/",
		},
	}
	ca := &externalplugins.ConfigAgent{}
	ca.Set(cfg)

	s := &Server{
		BotUser:        botUser,
		GitClient:      c,
		ConfigAgent:    ca,
		Push:           func(forkName, newBranch string, force bool) error { return nil },
		GitHubClient:   ghc,
		TokenGenerator: getSecret,
		Log:            logrus.StandardLogger().WithField("client", "cherrypicker"),
		Repos:          []github.Repo{{Fork: true, FullName: "ci-robot/bar"}},
	}

	if err := s.handlePullRequest(logrus.NewEntry(logrus.StandardLogger()), pr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var expectedFn = func(branch string) string {
		expectedTitle := fmt.Sprintf("This is a fix for Y (#2)[%s]", branch)
		expectedBody := "This is an automated cherry-pick of #2"
		expectedHead := fmt.Sprintf(botUser.Login+":"+cherryPickBranchFmt, 2, branch)
		var expectedLabels []string
		for _, label := range pr.PullRequest.Labels {
			expectedLabels = append(expectedLabels, label.Name)
		}
		var reviewers []string
		for _, reviewer := range pr.PullRequest.RequestedReviewers {
			reviewers = append(reviewers, reviewer.Login)
		}
		expectedAssignees := []string{"approver"}
		return fmt.Sprintf(expectedFmt, expectedTitle, expectedBody, expectedHead,
			branch, expectedLabels, reviewers, expectedAssignees)
	}

	if len(ghc.prs) != 2 {
		t.Fatalf("Expected %d PRs, got %d", 2, len(ghc.prs))
	}

	expectedBranches := []string{"release-1.5", "release-1.6"}
	seenBranches := make(map[string]struct{})
	for _, p := range ghc.prs {
		pr := prToString(p)
		if pr != expectedFn("release-1.5") && pr != expectedFn("release-1.6") {
			t.Errorf("Unexpected PR:\n%s\nExpected to target one of the following branches: %v\n%s",
				pr, expectedBranches, expectedFn("release-1.5"))
		}
		if pr == expectedFn("release-1.5") {
			seenBranches["release-1.5"] = struct{}{}
		}
		if pr == expectedFn("release-1.6") {
			seenBranches["release-1.6"] = struct{}{}
		}
	}
	if len(seenBranches) != 2 {
		t.Fatalf("Expected to see PRs for %d branches, got %d (%v)", 2, len(seenBranches), seenBranches)
	}
}

func TestCherryPickPRWithLabels(t *testing.T) {
	t.Parallel()
	testCherryPickPRWithLabels(localgit.New, t)
}

func TestCherryPickPRWithLabelsV2(t *testing.T) {
	t.Parallel()
	testCherryPickPRWithLabels(localgit.NewV2, t)
}

func testCherryPickPRWithLabels(clients localgit.Clients, t *testing.T) {
	lg, c, err := clients()
	if err != nil {
		t.Fatalf("Making localgit: %v", err)
	}
	defer func() {
		if err := lg.Clean(); err != nil {
			t.Errorf("Cleaning up localgit: %v", err)
		}
		if err := c.Clean(); err != nil {
			t.Errorf("Cleaning up client: %v", err)
		}
	}()
	if err := lg.MakeFakeRepo("foo", "bar"); err != nil {
		t.Fatalf("Making fake repo: %v", err)
	}
	if err := lg.AddCommit("foo", "bar", initialFiles); err != nil {
		t.Fatalf("Adding initial commit: %v", err)
	}
	if err := lg.CheckoutNewBranch("foo", "bar", "release-1.5"); err != nil {
		t.Fatalf("Checking out pull branch: %v", err)
	}
	if err := lg.CheckoutNewBranch("foo", "bar", "release-1.6"); err != nil {
		t.Fatalf("Checking out pull branch: %v", err)
	}

	pr := func(evt github.PullRequestEventAction) github.PullRequestEvent {
		return github.PullRequestEvent{
			Action: evt,
			PullRequest: github.PullRequest{
				User: github.User{
					Login: "developer",
				},
				Base: github.PullRequestBranch{
					Ref: "master",
					Repo: github.Repo{
						Owner: github.User{
							Login: "foo",
						},
						Name: "bar",
					},
				},
				Number:   2,
				Merged:   true,
				MergeSHA: new(string),
				Title:    "This is a fix for Y",
				RequestedReviewers: []github.User{
					{
						Login: "user1",
					},
				},
			},
		}
	}

	events := []github.PullRequestEventAction{github.PullRequestActionClosed, github.PullRequestActionLabeled}

	botUser := &github.UserData{Login: "ci-robot", Email: "ci-robot@users.noreply.github.com"}

	getSecret := func() []byte {
		return []byte("sha=abcdefg")
	}

	testCases := []struct {
		name        string
		labelPrefix string
		prLabels    []github.Label
		prComments  []github.IssueComment
	}{
		{
			name:        "Default label prefix",
			labelPrefix: externalplugins.DefaultCherryPickLabelPrefix,
			prLabels: []github.Label{
				{
					Name: "cherrypick/release-1.5",
				},
				{
					Name: "cherrypick/release-1.6",
				},
				{
					Name: "cherrypick/release-1.7",
				},
			},
		},
		{
			name:        "Custom label prefix",
			labelPrefix: "needs-cherry-pick-",
			prLabels: []github.Label{
				{
					Name: "needs-cherry-pick-release-1.5",
				},
				{
					Name: "needs-cherry-pick-release-1.6",
				},
				{
					Name: "needs-cherry-pick-release-1.7",
				},
			},
		},
		{
			name:        "No labels, label gets ignored",
			labelPrefix: "needs-cherry-pick-",
		},
	}

	for _, test := range testCases {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			for _, event := range events {
				evt := event
				t.Run(string(evt), func(t *testing.T) {
					ghc := &fghc{
						orgMembers: []github.TeamMember{
							{
								Login: "approver",
							},
							{
								Login: "merge-bot",
							},
							{
								Login: "developer",
							},
						},
						prComments: []github.IssueComment{
							{
								User: github.User{
									Login: "developer",
								},
								Body: "a review comment",
							},
							{
								User: github.User{
									Login: "developer",
								},
								Body: "/cherrypick release-1.5\r",
							},
						},
						prLabels: tc.prLabels,
						isMember: true,
						patch:    patch,
					}

					cfg := &externalplugins.Configuration{}
					cfg.TiCommunityCherrypicker = []externalplugins.TiCommunityCherrypicker{
						{
							Repos:       []string{"foo/bar"},
							LabelPrefix: tc.labelPrefix,
						},
					}
					ca := &externalplugins.ConfigAgent{}
					ca.Set(cfg)

					s := &Server{
						BotUser:        botUser,
						GitClient:      c,
						ConfigAgent:    ca,
						Push:           func(forkName, newBranch string, force bool) error { return nil },
						GitHubClient:   ghc,
						TokenGenerator: getSecret,
						Log:            logrus.StandardLogger().WithField("client", "cherrypicker"),
						Repos:          []github.Repo{{Fork: true, FullName: "ci-robot/bar"}},
					}

					if err := s.handlePullRequest(logrus.NewEntry(logrus.StandardLogger()), pr(evt)); err != nil {
						t.Fatalf("unexpected error: %v", err)
					}

					expectedFn := func(branch string) string {
						expectedTitle := fmt.Sprintf("This is a fix for Y (#2)[%s]", branch)
						expectedBody := "This is an automated cherry-pick of #2"
						expectedHead := fmt.Sprintf(botUser.Login+":"+cherryPickBranchFmt, 2, branch)
						var expectedLabels []string
						for _, label := range pr(evt).PullRequest.Labels {
							expectedLabels = append(expectedLabels, label.Name)
						}
						var reviewers []string
						for _, reviewer := range pr(evt).PullRequest.RequestedReviewers {
							reviewers = append(reviewers, reviewer.Login)
						}
						expectedAssignees := []string{"developer"}
						return fmt.Sprintf(expectedFmt, expectedTitle, expectedBody, expectedHead,
							branch, expectedLabels, reviewers, expectedAssignees)
					}

					expectedPRs := 2
					if len(tc.prLabels) == 0 {
						if evt == github.PullRequestActionLabeled {
							expectedPRs = 0
						} else {
							expectedPRs = 1
						}
					}
					if len(ghc.prs) != expectedPRs {
						t.Errorf("Expected %d PRs, got %d", expectedPRs, len(ghc.prs))
					}

					expectedBranches := []string{"release-1.5", "release-1.6"}
					seenBranches := make(map[string]struct{})
					for _, p := range ghc.prs {
						pr := prToString(p)
						if pr != expectedFn("release-1.5") && pr != expectedFn("release-1.6") {
							t.Errorf("Unexpected PR:\n%s\nExpected to target one of the following branches: %v", pr, expectedBranches)
						}
						if pr == expectedFn("release-1.5") {
							seenBranches["release-1.5"] = struct{}{}
						}
						if pr == expectedFn("release-1.6") {
							seenBranches["release-1.6"] = struct{}{}
						}
					}
					if len(seenBranches) != expectedPRs {
						t.Fatalf("Expected to see PRs for %d branches, got %d (%v)", expectedPRs, len(seenBranches), seenBranches)
					}
				})
			}
		})
	}
}

func TestCherryPickCreateIssue(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		org       string
		repo      string
		title     string
		body      string
		prNum     int
		labels    []string
		assignees []string
	}{
		{
			org:       "istio",
			repo:      "istio",
			title:     "brand new feature",
			body:      "automated cherry-pick",
			prNum:     2190,
			labels:    nil,
			assignees: []string{"clarketm"},
		},
		{
			org:       "kubernetes",
			repo:      "kubernetes",
			title:     "alpha feature",
			body:      "automated cherry-pick",
			prNum:     3444,
			labels:    []string{"new", "1.18"},
			assignees: nil,
		},
	}

	errMsg := func(field string) string {
		return fmt.Sprintf("GH issue %q does not match: \nexpected: \"%%v\" \nactual: \"%%v\"", field)
	}

	for _, tc := range testCases {
		ghc := &fghc{}

		s := &Server{
			GitHubClient: ghc,
		}

		if err := s.createIssue(logrus.WithField("test", t.Name()), tc.org, tc.repo, tc.title, tc.body, tc.prNum,
			nil, tc.labels, tc.assignees); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(ghc.issues) < 1 {
			t.Fatalf("Expected 1 GH issue to be created but got: %d", len(ghc.issues))
		}

		ghIssue := ghc.issues[len(ghc.issues)-1]

		if tc.title != ghIssue.Title {
			t.Fatalf(errMsg("title"), tc.title, ghIssue.Title)
		}

		if tc.body != ghIssue.Body {
			t.Fatalf(errMsg("body"), tc.title, ghIssue.Title)
		}

		if len(ghc.issues) != ghIssue.Number {
			t.Fatalf(errMsg("number"), len(ghc.issues), ghIssue.Number)
		}

		var actualAssignees []string
		for _, assignee := range ghIssue.Assignees {
			actualAssignees = append(actualAssignees, assignee.Login)
		}

		if !reflect.DeepEqual(tc.assignees, actualAssignees) {
			t.Fatalf(errMsg("assignees"), tc.assignees, actualAssignees)
		}

		var actualLabels []string
		for _, label := range ghIssue.Labels {
			actualLabels = append(actualLabels, label.Name)
		}

		if !reflect.DeepEqual(tc.labels, actualLabels) {
			t.Fatalf(errMsg("labels"), tc.labels, actualLabels)
		}

		cpFormat := fmt.Sprintf(commentFormat, tc.org, tc.repo, tc.prNum, "In response to a cherrypick label: %s")
		expectedComment := fmt.Sprintf(cpFormat, fmt.Sprintf("new issue created for failed cherrypick: #%d", ghIssue.Number))
		actualComment := ghc.comments[len(ghc.comments)-1]

		if expectedComment != actualComment {
			t.Fatalf(errMsg("comment"), expectedComment, actualComment)
		}
	}
}

func TestHandleLocks(t *testing.T) {
	t.Parallel()
	cfg := &externalplugins.Configuration{}
	cfg.TiCommunityCherrypicker = []externalplugins.TiCommunityCherrypicker{
		{
			Repos: []string{"org/repo"},
		},
	}
	ca := &externalplugins.ConfigAgent{}
	ca.Set(cfg)

	s := &Server{
		ConfigAgent:  ca,
		GitHubClient: &threadUnsafeFGHC{fghc: &fghc{}},
		BotUser:      &github.UserData{},
	}

	routine1Done := make(chan struct{})
	routine2Done := make(chan struct{})

	l := logrus.WithField("test", t.Name())
	pr := &github.PullRequest{
		Title:  "title",
		Body:   "body",
		Number: 0,
	}

	go func() {
		defer close(routine1Done)
		if err := s.handle(l, "", &github.IssueComment{}, "org", "repo", "targetBranch", pr); err != nil {
			t.Errorf("routine failed: %v", err)
		}
	}()
	go func() {
		defer close(routine2Done)
		if err := s.handle(l, "", &github.IssueComment{}, "org", "repo", "targetBranch", pr); err != nil {
			t.Errorf("routine failed: %v", err)
		}
	}()

	<-routine1Done
	<-routine2Done

	if actual := s.GitHubClient.(*threadUnsafeFGHC).orgRepoCountCalled; actual != 2 {
		t.Errorf("expected two EnsureFork calls, got %d", actual)
	}
}

func TestEnsureForkExists(t *testing.T) {
	botUser := &github.UserData{Login: "ci-robot", Email: "ci-robot@users.noreply.github.com"}

	ghc := &fghc{}

	cfg := &externalplugins.Configuration{}
	cfg.TiCommunityCherrypicker = []externalplugins.TiCommunityCherrypicker{
		{
			Repos: []string{"org/repo"},
		},
	}
	ca := &externalplugins.ConfigAgent{}
	ca.Set(cfg)

	s := &Server{
		BotUser:      botUser,
		ConfigAgent:  ca,
		GitHubClient: ghc,
		Repos:        []github.Repo{{Fork: true, FullName: "ci-robot/bar"}},
	}

	testCases := []struct {
		name     string
		org      string
		repo     string
		expected string
		errors   bool
	}{
		{
			name:     "Repo name does not change after ensured",
			org:      "whatever",
			repo:     "repo",
			expected: "repo",
			errors:   false,
		},
		{
			name:     "EnsureFork changes repo name",
			org:      "whatever",
			repo:     "changeme",
			expected: "changed",
			errors:   false,
		},
		{
			name:     "EnsureFork errors",
			org:      "whatever",
			repo:     "error",
			expected: "error",
			errors:   true,
		},
	}
	for _, test := range testCases {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			res, err := s.ensureForkExists(tc.org, tc.repo)
			if tc.errors && err == nil {
				t.Errorf("expected error, but did not get one")
			}
			if !tc.errors && err != nil {
				t.Errorf("expected no error, but got one")
			}
			if res != tc.expected {
				t.Errorf("expected %s but got %s", tc.expected, res)
			}
		})
	}
}

type threadUnsafeFGHC struct {
	*fghc
	orgRepoCountCalled int
}

func (tuf *threadUnsafeFGHC) EnsureFork(login, org, repo string) (string, error) {
	tuf.orgRepoCountCalled++
	return "", errors.New("that is enough")
}

func TestHelpProvider(t *testing.T) {
	enabledRepos := []config.OrgRepo{
		{Org: "org1", Repo: "repo"},
		{Org: "org2", Repo: "repo"},
	}
	cases := []struct {
		name               string
		config             *externalplugins.Configuration
		enabledRepos       []config.OrgRepo
		err                bool
		configInfoIncludes []string
		configInfoExcludes []string
	}{
		{
			name:               "Empty config",
			config:             &externalplugins.Configuration{},
			enabledRepos:       enabledRepos,
			configInfoExcludes: []string{"The current label prefix for cherry pick is: "},
		},
		{
			name: "All configs enabled",
			config: &externalplugins.Configuration{
				TiCommunityCherrypicker: []externalplugins.TiCommunityCherrypicker{
					{
						Repos:       []string{"org2/repo"},
						LabelPrefix: "needs-cherry-pick-",
						AllowAll:    true,
					},
				},
			},
			enabledRepos:       enabledRepos,
			configInfoIncludes: []string{"The current label prefix for cherry pick is: "},
		},
	}
	for _, testcase := range cases {
		tc := testcase
		t.Run(tc.name, func(t *testing.T) {
			epa := &externalplugins.ConfigAgent{}
			epa.Set(tc.config)

			helpProvider := HelpProvider(epa)
			pluginHelp, err := helpProvider(tc.enabledRepos)
			if err != nil && !tc.err {
				t.Fatalf("helpProvider error: %v", err)
			}
			for _, msg := range tc.configInfoExcludes {
				if strings.Contains(pluginHelp.Config["org2/repo"], msg) {
					t.Fatalf("helpProvider.Config error mismatch: got %v, but didn't want it", msg)
				}
			}
			for _, msg := range tc.configInfoIncludes {
				if !strings.Contains(pluginHelp.Config["org2/repo"], msg) {
					t.Fatalf("helpProvider.Config error mismatch: didn't get %v, but wanted it", msg)
				}
			}
		})
	}
}
