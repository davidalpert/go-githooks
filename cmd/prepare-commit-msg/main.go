package main

import (
	"bytes"
	"fmt"
	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/apex/log/handlers/text"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	Version = "n/a"
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
 */
type PrepareCommitMsgOptions struct {
	// positional args provided by git
	CommitMessageFile string
	CommitType        string
	CommitSHA         string

	// these are configuration options, set through env vars
	PrefixWithBranch           bool
	PrefixWithBranchExclusions []string
	PrefixWithBranchTemplate   string
	LogMinimumLevel            log.Level
	LogFile                    string
	Log                        *log.Entry
}

func NewOptions(logCtx *log.Entry) *PrepareCommitMsgOptions {
	if logCtx == nil {
		logCtx = log.WithFields(log.Fields{})
	}

	return &PrepareCommitMsgOptions{
		Log: logCtx,
		LogFile: getEnvOrDefaultString("GIT_COMMIT_MSG_LOG_FILE", fmt.Sprintf("%s.log", os.Args[0])),
		LogMinimumLevel: log.MustParseLevel(getEnvOrDefaultString("GIT_COMMIT_MSG_LOG_LEVEL", "error")),
	}
}

func (o *PrepareCommitMsgOptions) Prepare(args []string) error {
	// parse positional args
	numArgs := len(args)
	if !(2 <= numArgs && numArgs <= 3) {
		return fmt.Errorf("expected 'version' or 2 args or 3 args, got %d: %v", numArgs, args)
	}

	o.CommitMessageFile = args[0]
	o.CommitType = args[1]

	if numArgs > 2 {
		o.CommitSHA = args[2]
	}

	// import config options from environment
	o.PrefixWithBranch = getEnvOrDefaultBool("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME", false)
	o.PrefixWithBranchExclusions = getEnvOrDefaultStringSlice("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME_EXCLUSIONS", "master", "main", "dev", "develop")
	o.PrefixWithBranchTemplate = getEnvOrDefaultString("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME_TEMPLATE", "[%s]")

	o.Log = o.Log.WithFields(log.Fields{
		"commit.type": o.CommitType,
		"commit.sha": o.CommitSHA,
	})

	return nil
}

func (o *PrepareCommitMsgOptions) Execute() error {
	msg, err := ioutil.ReadFile(o.CommitMessageFile)
	if err != nil {
		return fmt.Errorf("could not read '%s': %v", o.CommitMessageFile, err)
	}

	if getEnvOrDefaultBool("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME", false) {
		excludedBranches := getEnvOrDefaultStringSlice("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME_EXCLUSIONS", "master", "main", "dev", "develop")
		branchTemplate := getEnvOrDefaultString("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME_TEMPLATE", "[%s]")
		currentBranch, err := determineCurrentBranch()
		if err != nil {
			o.Log.WithError(err).Debug("cannot find current branch")
		} else if !stringInSlice(excludedBranches, currentBranch) {
			msg = o.prependBranchName(msg, branchTemplate, currentBranch)
		}
	}

	coauthorMarkup, err := execAndCaptureOutput("list mob coauthors", "git", "mob-print")
	if err != nil {
		o.Log.WithError(err).Debug("could not list the mob")
	} else if coauthorMarkup != "" {
		msg = o.appendCoauthorMarkup(msg, coauthorMarkup)
	}

	err = os.WriteFile(o.CommitMessageFile, msg, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not write commit message '%s': %v", o.CommitMessageFile, err)
	}

	return nil
}

func determineCurrentBranch() (string, error) {
	currentBranch, err := execAndCaptureOutput("get current branch", "git", "branch", "--show-current")
	if err != nil {
		return "", err
	}
	if currentBranch != "" {
		return currentBranch, nil
	}

	branchList, err := execAndCaptureOutput("list branches", "git", "branch", "--list")
	if err != nil {
		return "", err
	}
	// * (no branch, rebasing feature/super-awesome-4)
	re := regexp.MustCompile("\\* \\(no branch, rebasing ([^)]+)\\)")
	match := re.FindStringSubmatch(branchList)
	if len(match) > 1 {
		return match[1], nil
	}

	return "", fmt.Errorf("could not find the current branch")
}

func (o *PrepareCommitMsgOptions) prependBranchName(msg []byte, template string, branch string) []byte {
	o.Log.WithField("branch", branch).Debug("adding branch prefix")
	if branch == "" {
		log.Debug("branch is empty, nothing to do")
		return msg // nothing to do
	}

	prefix := fmt.Sprintf(template, branch)
	prefixB := []byte(prefix)
	trimmedB := bytes.TrimSpace(msg)
	if !bytes.HasPrefix(trimmedB, prefixB) {
		o.Log.WithFields(log.Fields{
			"prefix": prefix,
		}).Debug("prepending message")
		return bytes.Join([][]byte{prefixB, trimmedB}, []byte(" "))
	}
	return msg
}

func (o *PrepareCommitMsgOptions) appendCoauthorMarkup(b []byte, coauthors string) []byte {
	re := regexp.MustCompile(`(?im)^co-authored-by: [^>]+>`)
	empty := []byte("")
	nl := []byte("\n")
	cleanedB := bytes.TrimSpace(re.ReplaceAll(b, empty))
	coauthorsB := bytes.TrimSpace([]byte(coauthors))
	if coauthors == "" {
		o.Log.Debug("no coauthors to add")
		return cleanedB
	}

	o.Log.Debug("found coauthors to add")
	if commentPos := strings.Index(string(cleanedB), "# "); commentPos > -1 {
		gitMessage := bytes.TrimSpace(cleanedB[0:commentPos])
		gitComments := cleanedB[commentPos:]
		o.Log.WithFields(log.Fields{
			"message": string(gitMessage),
			"comments": string(gitComments),
			"coauthors": coauthors,
		}).Debug("injecting coauthors")
		return bytes.Join([][]byte{gitMessage, nl, nl, coauthorsB, nl, gitComments}, empty)
	}
	o.Log.WithFields(log.Fields{
		"message": string(cleanedB),
		"coauthors": coauthors,
	}).Debug("appending coauthors")
	return bytes.Join([][]byte{cleanedB, nl, nl, coauthorsB}, empty)
}

func main() {
	//argsWithProg := os.Args
	argsWithoutProg := os.Args[1:]

	ctx := log.WithFields(log.Fields{
		"app":         "go-prepare-commit-msg",
		"app_version": Version,
	})
	o := NewOptions(ctx)

	if o.LogFile != "" {
		//fmt.Printf("logging to: %s\n", o.LogFile)
		f, err := os.OpenFile(o.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetHandler(text.New(f))
	} else {
		//fmt.Printf("logging to stdout\n")
		log.SetHandler(cli.New(os.Stdout))
	}
	log.SetLevel(o.LogMinimumLevel)

	if len(argsWithoutProg) == 1 && strings.EqualFold(argsWithoutProg[0], "version") {
		printVersion()
		return
	}

	if err := o.Prepare(argsWithoutProg); err != nil {
		log.WithError(err).Error("prepare options")
		fmt.Printf("%#v\n", err)
		os.Exit(1)
	}

	if err := o.Execute(); err != nil {
		log.WithError(err).Error("executing")
		fmt.Printf("%#v\n", err)
		os.Exit(1)
	}
}

// -- helpers -------------------------

func printVersion(errs ...error) {
	fmt.Printf("version: %s\n", Version)
	for _, e := range errs {
		fmt.Printf("- %v\n", e)
	}
}

/*
	cmd := exec.Command("tr", "a-z", "A-Z")
	cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
 */
func execAndCaptureOutput(cmdDescription string, cmdName string, arg ...string) (string, error) {
	cmd := exec.Command(cmdName, arg...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s failed: %v", cmdDescription, err)
	}

	return strings.TrimSpace(out.String()), nil
}

func getEnvOrDefaultBool(envKey string, defaultValue bool) bool {
	v := os.Getenv(envKey)
	if v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			panic(fmt.Errorf("failed parsing '%s' as a bool: %v", v, err))
		}
		return b
	}
	return defaultValue
}

func getEnvOrDefaultString(envKey string, defaultValue string) string {
	v := os.Getenv(envKey)
	if v != "" {
		return v
	}
	return defaultValue
}

func getEnvOrDefaultStringSlice(envKey string, defaults ...string ) []string {
	v := os.Getenv(envKey)
	if v != "" {
		return strings.Split(v, ",")
	}
	return defaults
}

func stringInSlice(s []string, v string) bool {
	for _, a := range s {
		if a == v {
			return true
		}
	}
	return false
}