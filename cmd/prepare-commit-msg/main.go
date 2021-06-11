package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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
	CommitMessageFile string
	CommitType        string
	CommitSHA         string
}

var (
	Version = "n/a"
)

func (o *PrepareCommitMsgOptions) Parse(args []string) error {
	numArgs := len(args)
	if !(2 <= numArgs && numArgs <= 3) {
		return fmt.Errorf("expected 'version' or 2 args or 3 args, got %d: %v", numArgs, args)
	}

	o.CommitMessageFile = args[0]
	o.CommitType = args[1]

	if numArgs > 2  {
		o.CommitSHA = args[2]
	}

	return nil
}

func (o *PrepareCommitMsgOptions) PrepareMessage() error {
	msg, err := ioutil.ReadFile(o.CommitMessageFile)
	if err != nil {
		return fmt.Errorf("could not read '%s': %v", o.CommitMessageFile, err)
	}

	if getEnvOrDefaultBool("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME", false) {
		excludedBranches := getEnvOrDefaultStringSlice("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME_EXCLUSIONS", "master", "main", "dev", "develop")
		branchTemplate := getEnvOrDefaultString("GIT_COMMIT_MSG_PREFIX_WITH_BRANCH_NAME_TEMPLATE", "[%s]")
		currentBranch, err := determineCurrentBranch()
		if err != nil {
			debugf("#v\n", err)
		} else if !stringInSlice(excludedBranches, currentBranch) {
			debugf("adding branch prefix [%s]\n", currentBranch)
			msg = prependBranchName(msg, branchTemplate, currentBranch)
		}
	}

	coauthorMarkup, err := execAndCaptureOutput("list mob coauthors", "git", "mob-print")
	if err != nil {
		debugf("%v\n", err)
	} else if coauthorMarkup != "" {
		debugf("adding coauthors\n")
		msg = appendCoauthorMarkup(msg, coauthorMarkup)
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

func prependBranchName(msg []byte, template string, branch string) []byte {
	if branch == "" {
		debugf("branch is empty, nothing to do")
		return msg // nothing to do
	}

	debugf("testing message '%s' for '%s' [%v]\n", string(msg), branch, strings.HasPrefix(string(msg), branch))
	prefix := fmt.Sprintf(template, branch)
	prefixB := []byte(prefix)
	trimmedB := bytes.TrimSpace(msg)
	if !bytes.HasPrefix(trimmedB, prefixB) {
		debugf("prepending message with '%s'\n", prefix)
		return bytes.Join([][]byte{prefixB, trimmedB}, []byte(" "))
	}
	return msg
}

func appendCoauthorMarkup(b []byte, coauthors string) []byte {
	re := regexp.MustCompile(`(?im)^co-authored-by: [^>]+>`)
	empty := []byte("")
	nl := []byte("\n")
	cleanedB := bytes.TrimSpace(re.ReplaceAll(b, empty))
	coauthorsB := bytes.TrimSpace([]byte(coauthors))
	if coauthors != "" {
		if commentPos := strings.Index(string(cleanedB), "# "); commentPos > -1 {
			gitMessage := bytes.TrimSpace(cleanedB[0:commentPos])
			gitComments := cleanedB[commentPos:]
			debugf("injecting \n%s\n in between \n%s\n and \n%s\n", coauthors, string(gitMessage), string(gitComments))
			return bytes.Join([][]byte{gitMessage, nl, nl, coauthorsB, nl, gitComments}, nl)
		}
		debugf("appending \n%s\n to \n%s\n", coauthors, string(cleanedB))
		return bytes.Join([][]byte{cleanedB, coauthorsB}, nl)
	}
	return cleanedB
}

func main() {
	//argsWithProg := os.Args
	argsWithoutProg := os.Args[1:]

	if len(argsWithoutProg) == 1 && argsWithoutProg[0] == "version" {
		printVersion()
		return
	}

	o := &PrepareCommitMsgOptions{}
	err := o.Parse(argsWithoutProg)
	if err != nil {
		panic(err)
	}

	err = o.PrepareMessage()
	if err != nil {
		panic(err)
	}
}

// -- helpers -------------------------

func printVersion(errs ...error) {
	fmt.Printf("version: %s\n", Version)
	for _, e := range errs {
		fmt.Printf("- %v\n", e)
	}
}

func debugf(format string, a ...interface{}) {
	if getEnvOrDefaultBool("DEBUG", false) {
		fmt.Printf(format, a...)
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