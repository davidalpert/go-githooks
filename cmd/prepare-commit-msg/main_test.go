package main

import (
	"bytes"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	approvals "github.com/approvals/go-approval-tests"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestRepoConfigUnmarshall(t *testing.T) {
	tests := []struct {
		name                     string
		configText               string
		prefixCommitMessage      bool
		prefixBranchExclusions   string
		PrefixWithBranchTemplate string
	}{
		{
			name: "s1",
			configText: `
[go-githooks "prepare-commit-message"]
       prefixWithBranch = true
`,
		},
		{
			name: "s2",
			configText: `
[go-githooks "prepare-commit-message"]
       prefixWithBranch = false
`,
		},
		{
			name: "s3",
			configText: `
[go-githooks "prepare-commit-message"]
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetHandler(text.New(&buf))
			log.SetLevel(log.DebugLevel)

			r, _ := git.Init(memory.NewStorage(), nil)
			cfg, _ := r.Config()

			err := cfg.Unmarshal([]byte(tt.configText))
			if err != nil {
				t.Errorf("unmarshalling sample config")
				t.Fail()
				return
			}

			approvals.VerifyJSONStruct(t, cfg.Raw.Sections)
		})
	}
}

func Test_overrideFromRepo(t *testing.T) {
	testcases := []struct {
		name       string
		configText string
		want       PrepareCommitMsgOptions
	}{
		{
			name: "s1",
			configText: `
[go-githooks "prepare-commit-message"]
    prefixWithBranch = true
`,
			want: PrepareCommitMsgOptions{
				PrefixWithBranch: true,
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := git.Init(memory.NewStorage(), memfs.New())
			cfg, _ := r.Config()
			err := cfg.Unmarshal([]byte(tt.configText))
			if err != nil {
				t.Errorf("unmarshalling sample config")
				return
			}

			o := NewOptions(r)
			o.overrideFromRepo()
			assert.Equal(t, tt.want.PrefixWithBranch, o.PrefixWithBranch)
			assert.Equal(t, tt.want.PrefixWithBranchTemplate, o.PrefixWithBranchTemplate)
			assert.Equal(t, tt.want.PrefixWithBranchExclusions, o.PrefixWithBranchExclusions)
		})
	}
}

func Test_appendCoauthorMarkup(t *testing.T) {
	tests := []struct {
		name            string
		rawMessage      string
		coauthorsMarkup string
		wantErr         bool
	}{
		{
			name:            "empty message empty coauthors",
			rawMessage:      ``,
			coauthorsMarkup: ``,
			wantErr:         false,
		},
		{
			name:            "empty message one coauthor",
			rawMessage:      ``,
			coauthorsMarkup: `Co-authored-by: Mal Reynolds <mal@serenity.com>`,
			wantErr:         false,
		},
		{
			name:            "empty message two coauthors",
			rawMessage:      ``,
			coauthorsMarkup: `Co-authored-by: Mal Reynolds <mal@serenity.com>
Co-authored-by: Zoe Washburne <zoe@serenity.com>`,
			wantErr:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &PrepareCommitMsgOptions{
				CommitMessageBytes:         []byte(tt.rawMessage),
				CoauthorsMarkupBytes:       []byte(tt.coauthorsMarkup),
			}
			if err := o.appendCoauthorMarkup(); (err != nil) != tt.wantErr {
				t.Errorf("appendCoauthorMarkup() error = %v, wantErr %v", err, tt.wantErr)
			}

			approvals.VerifyString(t, string(o.CommitMessageBytes))
		})
	}
}

func TestExecute(t *testing.T) {
	type testCaseArgs struct {
		name       string
		args       string
		rawMessage string
		branch     string
		coauthors  string
		want       string
		wantErr    bool
	}

	testsByRepoConfiguration := []struct {
		name                 string
		configText           string
		wantPrefixWithBranch bool
		testCases            []testCaseArgs
	}{
		{
			name: "r1",
			configText: `
[go-githooks "prepare-commit-message"]
        prefixWithBranch = true
`,
			testCases: []testCaseArgs{
				{
					name:       "empty msg on feature",
					args:       ".git/COMMIT_MSG",
					rawMessage: "",
					branch:     "FEAT-1",
					coauthors: `
Co-authored-by: Mal Reynolds <mal@serentiy.com>
`,
				},
				{
					name:       "existing msg on feature",
					args:       ".git/COMMIT_MSG message",
					rawMessage: "do something awesome",
					branch:     "FEAT-2",
					coauthors: `
Co-authored-by: Mal Reynolds <mal@serentiy.com>
`,
				},
				{
					name:       "existing msg on feature existing prefix",
					args:       ".git/COMMIT_MSG message",
					rawMessage: "[FEAT-3] do something awesome",
					branch:     "FEAT-3",
					coauthors: `
Co-authored-by: Mal Reynolds <mal@serentiy.com>
`,
				},
				{
					name:       "no message no coauthors",
					args:       ".git/COMMIT_MSG",
					rawMessage: "",
					branch:     "FEAT-4",
					coauthors:  "",
				},
				{
					name:       "no message no coauthors git comments",
					args:       ".git/COMMIT_MSG",
					rawMessage: `# git comments
# git comments
# git comments
`,
					branch:     "FEAT-5",
					coauthors:  "",
				},
			},
		},
		{
			name: "r2",
			configText: `
[go-githooks "prepare-commit-message"]
        prefixWithBranch = false
`,
			testCases: []testCaseArgs{},
		},
		{
			name: "r3",
			configText: `
[go-githooks "prepare-commit-message"]
`,
			testCases: []testCaseArgs{},
		},
	}
	for _, rr := range testsByRepoConfiguration {
		t.Run(rr.name, func(t *testing.T) {
			r, _ := git.Init(memory.NewStorage(), memfs.New())
			cfg, _ := r.Config()
			err := cfg.Unmarshal([]byte(rr.configText))
			if err != nil {
				t.Errorf("unmarshalling sample config")
				return
			}
			w, err := r.Worktree()
			if err != nil {
				t.Errorf("getting worktree: %v", err)
				return
			}
			_, err = w.Commit("empty root commit", &git.CommitOptions{})
			if err != nil {
				t.Errorf("creating root commit: %v", err)
				return
			}

			for _, tt := range rr.testCases {
				t.Run(tt.name, func(t *testing.T) {
					err = w.Checkout(&git.CheckoutOptions{
						Branch: plumbing.NewBranchReferenceName(tt.branch),
						Create: true,
					})
					if err != nil {
						t.Errorf("error creating test branch: %v", err)
						return
					}

					o := NewOptions(r)

					// inject these values from test data to simulate response from shelling out
					o.CommitMessageBytes = []byte(tt.rawMessage)
					o.CoauthorsMarkupBytes = []byte(tt.coauthors)

					cliArgs := strings.Split(tt.args, " ")
					err = o.Prepare(cliArgs)
					if err == nil {
						err = o.Execute()
					}

					if (err != nil) != tt.wantErr {
						t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
						return
					}

					approvals.VerifyString(t, string(o.CommitMessageBytes))
				})
			}
		})
	}
}
