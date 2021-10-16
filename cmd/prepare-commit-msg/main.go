package main

import (
	"bytes"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	Version = "n/a"
	empty = []byte("")
	space = []byte(" ")
	nl = []byte("\n")
)

/*
 * The prepare-commit-msg hook is run before the commit message editor is fired up but
 * after the default message is created. It lets you edit the default message before
 * the commit author sees it. This hook takes a few parameters: the path to the file
 * that holds the commit message so far, the type of commit, and the commit SHA-1 if
 * this is an amended commit. This hook generally isn’t useful for normal commits;
 * rather, it’s good for commits where the default message is auto-generated, such as
 * templated commit messages, merge commits, squashed commits, and amended commits.
 * You may use it in conjunction with a commit template to programmatically insert
 * information.
 *
 * reference: https://git-scm.com/docs/githooks#_prepare_commit_msg
 */
type PrepareCommitMsgOptions struct {
	// 1-3 positional args provided by git
	CommitMessageFile string
	Source            CommitMessageSource // optional
	CommitObject      string              // optional (required when Source is CommitSource)

	Repo *git.Repository

	// these are configuration options, set through env vars
	PrefixWithBranch           bool
	PrefixWithBranchExclusions []string
	PrefixWithBranchTemplate   string

	CommitMessageBytes   []byte
	CoauthorsMarkupBytes []byte
}

func NewOptions(repo *git.Repository) *PrepareCommitMsgOptions {
	return &PrepareCommitMsgOptions{
		Repo: repo,
	}
}

func (o *PrepareCommitMsgOptions) Prepare(args []string) error {
	// parse positional args
	numArgs := len(args)
	if !(1 <= numArgs && numArgs <= 3) {
		return fmt.Errorf("expected 'version' or 2 args or 3 args, got %d: %v", numArgs, args)
	}

	o.CommitMessageFile = args[0]

	if len(args) > 1 {
		o.Source = CommitMessageSourceFromString(args[1])
	}

	if o.Source == CommitSource {
		o.CommitObject = args[2]
	}

	_, err := o.Repo.ConfigScoped(config.GlobalScope)
	checkError("repoConfig", err)

	o.setDefaultOptions()
	o.overrideFromEnv() // TODO: replace with global .gitonfig
	o.overrideFromRepo() // HACK: for now, allow local repo config to override default config

	return nil
}

func (o *PrepareCommitMsgOptions) setDefaultOptions() {
	o.PrefixWithBranch = false
	o.PrefixWithBranchExclusions = []string{"main", "develop"}
	o.PrefixWithBranchTemplate = "[%s]"
}

func (o *PrepareCommitMsgOptions) overrideFromEnv() {
	o.PrefixWithBranch = getEnvOrDefaultBool("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME", o.PrefixWithBranch)
	o.PrefixWithBranchExclusions = getEnvOrDefaultStringSlice("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME_EXCLUSIONS", o.PrefixWithBranchExclusions...)
	o.PrefixWithBranchTemplate = getEnvOrDefaultString("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME_TEMPLATE", o.PrefixWithBranchTemplate)
}

func (o *PrepareCommitMsgOptions) overrideFromRepo() {
	cfg, err := o.Repo.ConfigScoped(config.GlobalScope)
	if err != nil {
		return
	}

	o.PrefixWithBranch = getRepoConfigOptionOrDefaultBool(cfg, "go-githooks", "prepare-commit-message", "prefixWithBranch", o.PrefixWithBranch)
	o.PrefixWithBranchExclusions = getRepoConfigOptionOrDefaultSlice(cfg, "go-githooks", "prepare-commit-message", "prefixBranchExclusions", o.PrefixWithBranchExclusions)
	o.PrefixWithBranchTemplate = getRepoConfigOptionOrDefaultString(cfg, "go-githooks", "prepare-commit-message", "prefixWithBranchTemplate", o.PrefixWithBranchTemplate)
}

func (o *PrepareCommitMsgOptions) Execute() error {
	if o.PrefixWithBranch {
		if err := o.prependBranchName(); err != nil {
			fmt.Printf("error prefixing branch name: %v\n", err)
		}
	}

	if len(o.CoauthorsMarkupBytes) > 0 {
		if err := o.appendCoauthorMarkup(); err != nil {
			fmt.Printf("error prefixing branch name: %v\n", err)
		}
	}

	return nil
}

func (o *PrepareCommitMsgOptions) prependBranchName() error {
	head, err := o.Repo.Head()
	if err != nil {
		return err
	}

	branchName := head.Name().Short()
	if branchName == "" {
		return nil
	}

	updated := make([]byte, 0)

	branchPrefix := strings.TrimSpace(fmt.Sprintf(o.PrefixWithBranchTemplate, branchName))
	trimmedMsg := bytes.TrimSpace(o.CommitMessageBytes)
	if bytes.HasPrefix(trimmedMsg, []byte("#")) {
		// inject to separate git comments from the prefix
		trimmedMsg = append(empty, bytes.Join([][]byte{ nl,
			nl,
			trimmedMsg,
		},empty)...)
	}
	if !bytes.HasPrefix(trimmedMsg, []byte(branchPrefix)) {
		updated = append(updated, bytes.Join([][]byte{
			[]byte(fmt.Sprintf(o.PrefixWithBranchTemplate, branchName)), []byte(" "), trimmedMsg, nl,
			nl,
		}, empty)...)
	} else {
		updated = append(updated, bytes.Join([][]byte{
			trimmedMsg, nl,
			nl,
		}, empty)...)
	}
	o.CommitMessageBytes = updated

	return nil
}

func (o *PrepareCommitMsgOptions) appendCoauthorMarkup() error {
	if len(o.CoauthorsMarkupBytes) == 0 {
		//fmt.Printf("no coauthors to add\n")
		return nil
	}
	//fmt.Printf("adding coauthors\n---\n%s\n---\n", string(o.CoauthorsMarkupBytes))
	re := regexp.MustCompile(`(?im)^co-authored-by: [^>]+>`)
	cleanedB := bytes.TrimSpace(re.ReplaceAll(o.CommitMessageBytes, empty))
	coauthorsB := bytes.TrimSpace(o.CoauthorsMarkupBytes)

	updated := make([]byte, 0)
	if commentPos := strings.Index(string(cleanedB), "# "); commentPos > -1 {
		gitMessage := bytes.TrimSpace(cleanedB[0:commentPos])
		gitComments := cleanedB[commentPos:]
		updated = append(updated, bytes.Join([][]byte{
			gitMessage, nl,
			nl,
			coauthorsB, nl,
			nl, gitComments,
		}, empty)...)
	} else {
		updated = append(updated, bytes.Join([][]byte{
			cleanedB, nl,
			nl,
			coauthorsB, nl,
			nl,
		}, empty)...)
	}
	//fmt.Printf("udpated:\n---\n%s\n---\n", string(updated))
	o.CommitMessageBytes = updated

	return nil
}

func (o *PrepareCommitMsgOptions) readCommitMessageFromDisk() error {
	msg, err := ioutil.ReadFile(o.CommitMessageFile)
	if os.IsNotExist(err) {
		msg = empty
	} else if err != nil {
		return fmt.Errorf("could not read '%s': %v", o.CommitMessageFile, err)
	}
	o.CommitMessageBytes = msg
	return nil
}

func (o *PrepareCommitMsgOptions) readCoauthorsMessage() error {
	coauthorMarkup, err := execAndCaptureOutput("list mob coauthors", "git", "mob-print")
	if err != nil {
		fmt.Printf("could not list the mob: %v\n", err)
	}
	o.CoauthorsMarkupBytes = []byte(coauthorMarkup)
	return nil
}

func main() {
	argsWithoutProg := os.Args[1:]
	numArgs := len(argsWithoutProg)

	if numArgs == 1 {
		switch argsWithoutProg[0] {
		case "version":
			printVersion()
			return
		case "help":
			printHelp()
			return
		}
	}

	repoDir := getEnvOrDefaultString("PREPARE_COMMIT_MESSAGE_REPO_DIR", ".")
	absDir, _ := filepath.Abs(repoDir)
	//fmt.Printf("opening git config @ '%s'\n", absDir)
	repo, err := git.PlainOpen(absDir)
	if err == git.ErrRepositoryNotExists {
		err = fmt.Errorf("could not find repo at '%s' (resovled to: %s): %v", repoDir, absDir, err)
	}
	checkError("read git repo", err)

	o := NewOptions(repo)

	err = o.Prepare(argsWithoutProg)
	checkError("prepare options", err)

	err = o.readCommitMessageFromDisk()
	checkError("readCommitMessage", err)

	err = o.readCoauthorsMessage()
	checkError("readCoauthorsMessage", err)

	err = o.Execute()
	checkError("executing", err)

	//o.CommitMessageBytes = append(o.CommitMessageBytes, bytes.Join([][]byte{
	//	space, []byte("foo"), nl,
	//}, empty)...)

	err = os.WriteFile(o.CommitMessageFile, o.CommitMessageBytes, os.ModePerm)
	if err != nil {
		checkError("writing file", fmt.Errorf("could not write commit message '%s': %v", o.CommitMessageFile, err))
	}
}

func printVersion(errs ...error) {
	fmt.Printf("version: %s\n", Version)
	for _, e := range errs {
		fmt.Printf("- %v\n", e)
	}
}

func printHelp() {
	fmt.Printf("help: %s\n", Version)
	fmt.Printf(`
configure go-githooks per-repo in .git/config:

[go-githooks "prepare-commit-message"]
    prefixWithBranch = false
    prefixWithBranchTemplate = [%%s]
    prefixBranchExclusions = main,develop

`)
}
