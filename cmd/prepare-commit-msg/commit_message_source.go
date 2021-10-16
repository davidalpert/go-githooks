package main

type CommitMessageSource int

const (
	UnknownSource  CommitMessageSource = iota
	EmptySource                    // source not provided
	MessageSource                    // if a -m or -F option was given
	TemplateSource                    // if a -t option was given or the configuration option commit.template is set
	MergeSource                    // if the commit is a merge or a .git/MERGE_MSG file exists
	SquashSource                    // if a .git/SQUASH_MSG file exists
	CommitSource                    // followed by a commit object name (if a -c, -C or --amend option was given)
)

func CommitMessageSourceFromString(s string) CommitMessageSource {
	switch s {
	case "":
		return EmptySource
	case "message":
		return MessageSource
	case "template":
		return TemplateSource
	case "merge":
		return MergeSource
	case "squash":
		return SquashSource
	case "commit":
		return CommitSource
	}
	return UnknownSource
}

func (s CommitMessageSource) String() string {
	switch s {
	case EmptySource:
		return ""
	case MessageSource:
		return "message"
	case TemplateSource:
		return "template"
	case MergeSource:
		return "merge"
	case SquashSource:
		return "squash"
	case CommitSource:
		return "commit"
	}
	return "unknown"
}
