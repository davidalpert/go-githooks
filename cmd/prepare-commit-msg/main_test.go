package main

import (
	"bytes"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	approvals "github.com/approvals/go-approval-tests"
	"testing"
)

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