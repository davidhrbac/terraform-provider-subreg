package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type block struct {
	lines   []string
	kind    int
	sortKey string
	name    string
}

var resourceLineRE = regexp.MustCompile(`^resource\s+"([^"]+)"\s+"([^"]+)"\s*\{$`)
var importCommentRE = regexp.MustCompile(`^# __generated__ by Terraform from "([^"]+)"$`)

func main() {
	var inputPath string
	var outputPath string
	flag.StringVar(&inputPath, "input", "", "input config path")
	flag.StringVar(&outputPath, "output", "", "output config path")
	flag.Parse()

	if inputPath == "" || outputPath == "" {
		fatal("missing -input or -output")
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		fatal(err.Error())
	}

	output, err := sortGeneratedConfig(string(data))
	if err != nil {
		fatal(err.Error())
	}

	if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
		fatal(err.Error())
	}
}

func sortGeneratedConfig(input string) (string, error) {
	lines := strings.Split(strings.TrimRight(input, "\n"), "\n")
	if len(lines) == 0 {
		return input, nil
	}

	firstBlock := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "# __generated__ by Terraform from ") {
			firstBlock = i
			break
		}
	}
	if firstBlock == -1 {
		return input, nil
	}

	header := append([]string(nil), lines[:firstBlock]...)
	header = trimTrailingBlankLines(header)
	blocks, err := parseBlocks(lines[firstBlock:])
	if err != nil {
		return "", err
	}

	sort.SliceStable(blocks, func(i, j int) bool {
		if blocks[i].kind != blocks[j].kind {
			return blocks[i].kind < blocks[j].kind
		}
		if blocks[i].sortKey != blocks[j].sortKey {
			return blocks[i].sortKey < blocks[j].sortKey
		}
		return blocks[i].name < blocks[j].name
	})

	var out strings.Builder
	if len(header) > 0 {
		out.WriteString(strings.Join(header, "\n"))
		out.WriteString("\n\n")
	}
	for i, b := range blocks {
		out.WriteString(strings.Join(b.lines, "\n"))
		if i < len(blocks)-1 {
			out.WriteString("\n\n")
		} else {
			out.WriteString("\n")
		}
	}

	return out.String(), nil
}

func parseBlocks(lines []string) ([]block, error) {
	var blocks []block
	for i := 0; i < len(lines); {
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		if i >= len(lines) {
			break
		}

		start := i
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			i++
		}
		blockLines := append([]string(nil), lines[start:i]...)
		parsed, err := classifyBlock(blockLines)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, parsed)
	}
	return blocks, nil
}

func classifyBlock(lines []string) (block, error) {
	if len(lines) < 2 {
		return block{}, errors.New("invalid generated config block")
	}
	commentMatch := importCommentRE.FindStringSubmatch(strings.TrimSpace(lines[0]))
	if commentMatch == nil {
		return block{}, fmt.Errorf("unexpected generated config comment: %q", lines[0])
	}
	resourceMatch := resourceLineRE.FindStringSubmatch(strings.TrimSpace(lines[1]))
	if resourceMatch == nil {
		return block{}, fmt.Errorf("unexpected generated config resource line: %q", lines[1])
	}

	kind := resourceKindPriority(resourceMatch[1])
	return block{
		lines:   lines,
		kind:    kind,
		sortKey: blockSortKey(resourceMatch[1], commentMatch[1]),
		name:    resourceMatch[2],
	}, nil
}

func resourceKindPriority(resourceType string) int {
	switch resourceType {
	case "subreg_domain":
		return 0
	case "subreg_dns_zone":
		return 1
	case "subreg_dns_record":
		return 2
	default:
		return 3
	}
}

func blockSortKey(resourceType, importID string) string {
	switch resourceType {
	case "subreg_domain", "subreg_dns_zone":
		return importID
	case "subreg_dns_record":
		parts := strings.Split(importID, ":")
		if len(parts) == 2 {
			if id, err := strconv.Atoi(parts[1]); err == nil {
				return fmt.Sprintf("%020d", id)
			}
		}
		return importID
	default:
		return importID
	}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func trimTrailingBlankLines(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[:end]
}
