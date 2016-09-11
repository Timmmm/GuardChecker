package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ScanLinesKeepEnding is a split function for a Scanner that returns each line of
// text, *without* stripping them of any trailing end-of-line marker.
// The returned line may be empty. The end-of-line marker is one optional carriage
// return followed by one mandatory newline. In regular expression notation, it is `\r?\n`.
// The last non-empty line of input will be returned even if it has no newline.
func ScanLinesKeepEnding(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[0 : i+1], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

// readLines reads a whole file into memory and returns a slice of its lines.
// Line ending characters are not stripped.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Split(ScanLinesKeepEnding)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLines writes the lines to the given file. Line ending characters must
// be included.
func writeLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprint(w, line)
	}
	return w.Flush()
}

// These are some *very* simple regexes for detecting the include guards.
var ifdefcppRegex *regexp.Regexp = regexp.MustCompile(`#ifdef\s+__cplusplus(\s+.*)?`)
var externcRegex *regexp.Regexp = regexp.MustCompile(`extern\s+"C"(\s+.*)?`)
var endifRegex *regexp.Regexp = regexp.MustCompile(`#endif(\s+.*)?`)
var closeblockRegex *regexp.Regexp = regexp.MustCompile(`}.*`)
var defineRegex *regexp.Regexp = regexp.MustCompile(`#define\s+.*`)

// fileHasCppIncludeGuards does a rough check for C++ include guards. It might
// not always be right if you write them in a weird way (it uses fallible regexes).
func fileHasCppIncludeGuards(lines []string) bool {
	// Search for
	//
	// #ifdef __cplusplus
	// extern "C" {
	// #endif
	//
	// and
	//
	// #ifdef __cplusplus
	// }
	// #endif

	// To keep things simple, we just search for those lines, in that order, ignoring all other lines, using shitty regexes.
	// This will allow false positives, but you'll really only get them if you write extremely strange code.

	regexes := []*regexp.Regexp{
		ifdefcppRegex,
		externcRegex,
		endifRegex,
		ifdefcppRegex,
		closeblockRegex,
		endifRegex,
	}

	i := 0
	for _, line := range lines {
		if regexes[i].MatchString(strings.TrimSpace(line)) {
			i += 1
		}
		if i >= len(regexes) {
			return true
		}
	}
	return false
}

// addCppIncludeGuards tries to insert C++ include guards just inside the
// normal header include guards. It does this in a really naive way so check the
// results! Basically it inserts them after the first #define, and before the last
// #endif
//
// It preserves existing line endings but always uses '\n' on newly inserted lines.
func addCppIncludeGuards(lines []string) (modifiedLines []string, err error) {

	modifiedLines = make([]string, 0, len(lines)+8)

	// Find the first line matching the #define regex, and the last line matching
	// the #endif regex.

	firstDefine := -1
	for i := 0; i < len(lines); i += 1 {
		if defineRegex.MatchString(strings.TrimSpace(lines[i])) {
			firstDefine = i
			break
		}
	}

	lastEndif := -1
	for i := len(lines) - 1; i >= 0; i -= 1 {
		if endifRegex.MatchString(strings.TrimSpace(lines[i])) {
			lastEndif = i
			break
		}
	}

	if firstDefine == -1 {
		err = errors.New("Couldn't find first #define")
		return
	}

	if lastEndif == -1 {
		err = errors.New("Couldn't find last #endif")
		return
	}

	for i, line := range lines {
		if i == firstDefine {
			// Add a couple of lines afterwards...
			modifiedLines = append(modifiedLines, line)
			modifiedLines = append(modifiedLines, "\n")
			modifiedLines = append(modifiedLines, "#ifdef __cplusplus\n")
			modifiedLines = append(modifiedLines, "extern \"C\" {\n")
			modifiedLines = append(modifiedLines, "#endif\n")
		} else if i == lastEndif {
			modifiedLines = append(modifiedLines, "#ifdef __cplusplus\n")
			modifiedLines = append(modifiedLines, "}\n")
			modifiedLines = append(modifiedLines, "#endif\n")
			modifiedLines = append(modifiedLines, "\n")
			modifiedLines = append(modifiedLines, line)
		} else {
			modifiedLines = append(modifiedLines, line)
		}
	}
	return
}

// fixFile checks if a file is a C header (i.e. ends in .h). If so
// it scans the file for C++ include guards and adds them if absent.
func fixFile(path string, info os.FileInfo, err error) error {
	// If `err` is not nil then there was an error walking to the file
	// named by `path`.

	if strings.HasSuffix(info.Name(), ".h") {
		lines, err := readLines(path)
		if err != nil {
			log.Println("Error reading file:", err)
			return nil
		}

		hasGuards := fileHasCppIncludeGuards(lines)

		if !hasGuards {
			log.Println("Adding guards to:", path)
			modifiedLines, err := addCppIncludeGuards(lines)
			if err != nil {
				log.Println("Error adding include guards:", err)
				return nil
			}

			err = writeLines(modifiedLines, path)
			if err != nil {
				log.Println("Error writing to file:", err)
				return nil
			}
		}
	}

	// If we return an error, processing stops.
	return nil
}

func main() {
	flag.Parse()
	root := flag.Arg(0)

	err := filepath.Walk(root, fixFile)
	if err != nil {
		log.Println("Error scanning files:", err)
	}
}
