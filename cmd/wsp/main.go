package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Sorune/wsp/internal/config"
	"github.com/Sorune/wsp/internal/inspect"
	"github.com/Sorune/wsp/internal/lens"
	"github.com/Sorune/wsp/internal/model"
	"github.com/Sorune/wsp/internal/present"
)

const version = "0.1.0-dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "STATUS: BLOCKED\nREASON: %s\n", err)
		os.Exit(exitCode(err))
	}
}

type cliError struct {
	category, message string
	code              int
}

func (e cliError) Error() string { return e.category + ": " + e.message }
func fail(category, message string, code int) error {
	return cliError{category: category, message: message, code: code}
}
func exitCode(err error) int {
	if e, ok := err.(cliError); ok {
		return e.code
	}
	return 1
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		usage()
		return nil
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			return fail("USAGE", "version takes no arguments", 2)
		}
		fmt.Printf("WSP\nVERSION: %s\nRUNTIME: Go semantic core + POSIX shell front door\n", version)
		return nil
	case "doctor":
		return doctor(args[1:])
	case "status":
		return inspectCommand("status", callerPath(), args[1:], false)
	case "repo":
		if len(args) < 2 || args[1] != "inspect" {
			return fail("USAGE", "repo inspect [path]", 2)
		}
		path := "."
		flags := []string{}
		for _, arg := range args[2:] {
			if strings.HasPrefix(arg, "--") {
				flags = append(flags, arg)
			} else if path == "." {
				path = arg
			} else {
				return fail("USAGE", "repo inspect [path] [--json]", 2)
			}
		}
		if path == "." {
			path = callerPath()
		}
		return inspectCommand("repo.inspect", path, flags, false)
	case "inspect":
		path, flags, err := targetAndFlags(args[1:])
		if err != nil {
			return err
		}
		if path == "." {
			path = callerPath()
		}
		return inspectCommand("inspect", path, flags, true)
	case "lens":
		return lensCommand(args[1:])
	case "init":
		return initCommand(args[1:])
	default:
		return fail("USAGE", "unknown command: "+args[0], 2)
	}
}

func usage() {
	fmt.Print("Usage: wsp <command> [args]\n\nCommands:\n  init [path]\n  inspect [path] [--json]\n  repo inspect [path]\n  lens tree [path] --axis logical|session [--json]\n  status\n  doctor\n  version\n")
}

func doctor(args []string) error {
	if len(args) != 0 {
		return fail("USAGE", "doctor takes no arguments", 2)
	}
	missing := []string{}
	for _, name := range []string{"git"} {
		if _, err := execLookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	fmt.Println("WSP DOCTOR")
	if len(missing) > 0 {
		fmt.Println("STATUS: BLOCKED")
		return fail("INTERNAL_FAILURE", "missing dependency: "+strings.Join(missing, ","), 1)
	}
	fmt.Println("STATUS: PASS")
	fmt.Println("MUTATION: NONE")
	fmt.Println("RUNTIME: Go semantic core")
	return nil
}

// Kept behind a tiny function so the command layer remains easy to test.
func execLookPath(name string) (string, error) { return exec.LookPath(name) }

func inspectCommand(command, path string, flags []string, allowAxis bool) error {
	jsonOut := hasFlag(flags, "--json")
	for _, f := range flags {
		if f != "--json" && (!allowAxis || !strings.HasPrefix(f, "--axis=")) {
			return fail("USAGE", "unsupported option: "+f, 2)
		}
	}
	doc, err := inspect.Repository(path)
	if err != nil {
		return failFor(err)
	}
	doc.Command = command
	return emit(doc, jsonOut)
}

func lensCommand(args []string) error {
	if len(args) < 2 || args[0] != "tree" {
		return fail("USAGE", "lens tree [path] --axis logical|session [--json]", 2)
	}
	path := "."
	axis := ""
	jsonOut := false
	for i := 1; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			jsonOut = true
		case args[i] == "--axis" && i+1 < len(args):
			i++
			axis = args[i]
		case strings.HasPrefix(args[i], "--axis="):
			axis = strings.TrimPrefix(args[i], "--axis=")
		case strings.HasPrefix(args[i], "--"):
			return fail("USAGE", "unsupported option: "+args[i], 2)
		case path == ".":
			path = args[i]
		default:
			return fail("USAGE", "too many targets", 2)
		}
	}
	if axis == "" {
		return fail("USAGE", "lens tree requires --axis logical|session", 2)
	}
	doc, tree, err := inspect.Tree(path, axis)
	if err != nil {
		return failFor(err)
	}
	doc.Command = "lens.tree"
	doc.Projection = tree
	return emitTree(doc, tree, jsonOut)
}

func initCommand(args []string) error {
	if len(args) > 1 {
		return fail("USAGE", "init [path]", 2)
	}
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	if root == "." {
		root = callerPath()
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return fail("INTERNAL_FAILURE", err.Error(), 1)
	}
	if _, err := os.Stat(config.Path(root)); err == nil {
		return fail("INVALID_CONFIGURATION", "manifest already exists", 4)
	}
	m, err := config.Discover(root)
	if err != nil {
		return fail("INTERNAL_FAILURE", err.Error(), 1)
	}
	observed, err := inspect.Repository(root)
	if err != nil {
		return failFor(err)
	}
	for _, entity := range observed.Entities {
		if entity.Kind == "REPOSITORY" {
			m.Repositories[0].ID = entity.ID
			m.Repositories[0].Name = entity.Name
			break
		}
	}
	if len(m.Repositories) == 1 {
		m.Relations = []model.Relation{{ID: "contains:" + m.WorkspaceID + ":" + m.Repositories[0].ID, Type: "contains", From: m.WorkspaceID, To: m.Repositories[0].ID, Provenance: model.ProvenanceConfig, Reason: "explicitly initialized workspace relation"}}
	}
	if err := config.Write(root, m); err != nil {
		return failFor(err)
	}
	fmt.Printf("STATUS: INITIALIZED\nMANIFEST: %s\nMUTATION: WSP_CONFIG_ONLY\n", config.Path(root))
	return nil
}

func callerPath() string {
	if value := os.Getenv("WSP_CALLER_PWD"); value != "" {
		return value
	}
	return "."
}

func targetAndFlags(args []string) (string, []string, error) {
	path := "."
	flags := []string{}
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			flags = append(flags, a)
		} else if path == "." {
			path = a
		} else {
			return "", nil, fail("USAGE", "one target allowed", 2)
		}
	}
	return path, flags, nil
}

func emit(doc model.Document, jsonOut bool) error {
	if jsonOut {
		b, err := present.JSON(doc)
		if err != nil {
			return fail("INTERNAL_FAILURE", err.Error(), 1)
		}
		fmt.Println(string(b))
		return nil
	}
	fmt.Print(present.Document(doc))
	return nil
}

func emitTree(doc model.Document, tree lens.Tree, jsonOut bool) error {
	if jsonOut {
		b, err := present.JSON(doc)
		if err != nil {
			return fail("INTERNAL_FAILURE", err.Error(), 1)
		}
		fmt.Println(string(b))
		return nil
	}
	fmt.Print(present.Tree(tree))
	return nil
}

func hasFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

func failFor(err error) error {
	msg := err.Error()
	category := "INTERNAL_FAILURE"
	code := 1
	for _, c := range []string{"TARGET_NOT_FOUND", "TARGET_UNAVAILABLE", "INVALID_CONFIGURATION", "INVALID_RELATION", "INVALID_REQUEST"} {
		if strings.HasPrefix(msg, c) {
			category = c
			switch c {
			case "INVALID_REQUEST":
				code = 2
			case "INVALID_CONFIGURATION", "INVALID_RELATION":
				code = 4
			default:
				code = 3
			}
			if i := strings.Index(msg, ": "); i >= 0 {
				msg = msg[i+2:]
			}
			break
		}
	}
	return fail(category, msg, code)
}
