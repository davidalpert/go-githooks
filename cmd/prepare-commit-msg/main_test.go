package main

import (
	"bytes"
	"fmt"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	approvals "github.com/approvals/go-approval-tests"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestRepoConfigOptions(t *testing.T) {
	o := NewOptions(nil)
	o.setDefaultOptions()
	repoPath, _ := filepath.Abs(getEnvOrDefaultString("GITHOOKS_TEST_REPO_PATH", "."))
	fmt.Printf("testing agains repo path: %s\n", repoPath)
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		t.Errorf("error opening repo: %v", err)
		t.Fail()
	} else if r == nil {
		t.Errorf("error opening repo: %v", err)
		t.Fail()
	}
	c, err := r.Config()
	if err != nil {
		t.Errorf("error reading config: %v", err)
		t.Fail()
	}
	o.overrideFromRepo(c)

	assert.True(t, o.PrefixWithBranch)
}

func TestAppendCoauthors(t *testing.T) {
	var buf bytes.Buffer
	log.SetHandler(text.New(&buf))
	log.SetLevel(log.DebugLevel)

	tests := []struct {
		name             string
		msg              string
		prefixWithBranch bool
		branch           string
		coauthors        string
		wantErr          bool
	}{
		{
			name: "simple-message",
			msg: `single line msg`,
			prefixWithBranch: false,
			coauthors: `Co-authored-by: Zoe Washburne <zoe.washburne@serenity.org>`,
		},
		{
			name: "simple-message-two-authors",
			msg: `single line msg`,
			prefixWithBranch: false,
			coauthors: `Co-authored-by: Zoe Washburne <zoe.washburne@serenity.org>
Co-authored-by: Sheppard Book <sheppard.book@serenity.org>`,
		},
		{
			name: "simple-message-has-existing-coauthor",
			msg: `single line msg

Co-authored-by: Sheppard Book <sheppard.book@serenity.org>
`,
			prefixWithBranch: false,
			coauthors: `Co-authored-by: Zoe Washburne <zoe.washburne@serenity.org>`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(tt *testing.T) {
			o := NewOptions(nil)
			msg := []byte(c.msg)
			b := o.appendCoauthorMarkup(msg, c.coauthors)

			approvals.VerifyString(tt, string(b))
		})
	}
}

func TestPrependBranchName(t *testing.T) {
	var buf bytes.Buffer
	log.SetHandler(text.New(&buf))
	log.SetLevel(log.DebugLevel)

	tests := []struct {
		name             string
		msg              string
		prefixWithBranch bool
		template         string
		branch           string
		wantErr          bool
	}{
		{
			name: "empty-message",
			msg: ``,
			prefixWithBranch: true,
			template: "[%s]",
			branch: "GH-123",
		},
		{
			name: "existing-message",
			msg: `do something`,
			prefixWithBranch: true,
			template: "[%s]",
			branch: "GH-123",
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(tt *testing.T) {
			o := NewOptions(nil)
			msg := []byte(c.msg)
			b := o.prependBranchName(msg, c.template, c.branch)

			approvals.VerifyString(tt, string(b))
		})
	}
}
