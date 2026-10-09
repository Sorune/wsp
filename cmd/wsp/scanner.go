package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/Sorune/wsp/internal/scanner"
	"github.com/Sorune/wsp/internal/scannerdiff"
	"github.com/Sorune/wsp/internal/scannerview"
)

type scannerScanOptions struct {
	Path        string
	SubjectID   string
	Artifact    string
	SourceRoots []string
}

func scannerCommand(args []string) error {
	if len(args) == 0 {
		return fail("USAGE", "scanner requires scan|view|compare", 2)
	}
	switch args[0] {
	case "scan":
		return scannerScanCommand(args[1:])
	case "view":
		return scannerViewCommand(args[1:])
	case "compare":
		return scannerCompareCommand(args[1:])
	default:
		return fail("USAGE", "scanner requires scan|view|compare", 2)
	}
}

func parseScannerScanArgs(args []string) (scannerScanOptions, error) {
	o := scannerScanOptions{Artifact: "both"}
	var paths []string
	seenSubject, seenArtifact := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") {
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
				return o, fmt.Errorf("scanner scan option %s needs a value", a)
			}
			v := args[i+1]
			i++
			switch a {
			case "--subject-id":
				if seenSubject {
					return o, fmt.Errorf("scanner scan repeats --subject-id")
				}
				seenSubject = true
				o.SubjectID = v
			case "--source-root":
				o.SourceRoots = append(o.SourceRoots, v)
			case "--artifact":
				if seenArtifact {
					return o, fmt.Errorf("scanner scan repeats --artifact")
				}
				seenArtifact = true
				o.Artifact = v
			default:
				return o, fmt.Errorf("scanner scan does not support option %s", a)
			}
		} else {
			paths = append(paths, a)
		}
	}
	if len(paths) != 1 {
		return o, fmt.Errorf("scanner scan needs exactly one <repository>")
	}
	if !seenSubject {
		return o, fmt.Errorf("scanner scan requires --subject-id ID")
	}
	if o.Artifact != "snapshot" && o.Artifact != "candidates" && o.Artifact != "both" {
		return o, fmt.Errorf("invalid --artifact %q", o.Artifact)
	}
	o.Path = targetPath(paths[0])
	return o, nil
}

func scannerScanCommand(args []string) error {
	o, err := parseScannerScanArgs(args)
	if err != nil {
		return fail("USAGE", err.Error(), 2)
	}
	config := scanner.Config{SourceRoots: o.SourceRoots}
	batch, err := scanner.Observe(scanner.Request{Path: o.Path, SubjectID: o.SubjectID, Config: config})
	if err != nil {
		return fail("INVALID_REQUEST", "scanner observation failed: "+err.Error(), 2)
	}
	snapshot, err := scanner.BuildSnapshot(batch, config)
	if err != nil {
		return fail("INTERNAL_FAILURE", "scanner snapshot validation failed: "+err.Error(), 1)
	}
	var candidates scanner.DerivedCandidateSet
	if o.Artifact != "snapshot" {
		candidates, err = scanner.Derive(snapshot)
		if err != nil {
			return fail("INTERNAL_FAILURE", "scanner derivation failed: "+err.Error(), 1)
		}
	}
	var data []byte
	switch o.Artifact {
	case "snapshot":
		data, err = scanner.SerializeSnapshot(snapshot)
	case "candidates":
		data, err = scanner.SerializeCandidates(candidates)
	default:
		data, err = scanner.SerializeBundle(snapshot, candidates)
	}
	if err != nil {
		return fail("INTERNAL_FAILURE", "scanner serialization failed: "+err.Error(), 1)
	}
	fmt.Print(string(data))
	return nil
}

func normalizeScannerFileArgs(args []string, names ...string) ([]string, error) {
	pathFlag := map[string]bool{}
	for _, name := range names {
		pathFlag[name] = true
	}
	out := append([]string(nil), args...)
	for i := 0; i < len(out); i++ {
		if !pathFlag[out[i]] {
			continue
		}
		if i+1 >= len(out) || out[i+1] == "" || strings.HasPrefix(out[i+1], "--") {
			return nil, fmt.Errorf("%s needs a path", out[i])
		}
		i++
		out[i] = targetPath(out[i])
	}
	return out, nil
}

func scannerViewCommand(args []string) error {
	normalized, err := normalizeScannerFileArgs(args, "--input")
	if err != nil {
		return fail("USAGE", err.Error(), 2)
	}
	var stdout, stderr bytes.Buffer
	code := scannerview.Run(normalized, &stdout, &stderr)
	if code != 0 {
		return fail("INVALID_REQUEST", strings.TrimSpace(stderr.String()), 2)
	}
	fmt.Print(stdout.String())
	return nil
}

func scannerCompareCommand(args []string) error {
	normalized, err := normalizeScannerFileArgs(args, "--baseline", "--current", "--reference")
	if err != nil {
		return fail("USAGE", err.Error(), 2)
	}
	var stdout, stderr bytes.Buffer
	code := scannerdiff.Run(normalized, &stdout, &stderr)
	if code != 0 {
		return fail("INVALID_REQUEST", strings.TrimSpace(stderr.String()), 2)
	}
	fmt.Print(stdout.String())
	return nil
}
