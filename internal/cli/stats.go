package cli

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

type statsUsageLine struct {
	Client       string `json:"client"`
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
}

// Stats summarizes a redacted usage JSONL file.
func Stats(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	path := fs.String("path", "", "path to redacted usage JSONL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *path == "" {
		return fmt.Errorf("usage: codexpass stats --path PATH")
	}
	f, err := os.Open(*path)
	if err != nil {
		return err
	}
	defer f.Close()

	type agg struct {
		Requests int64
		Input    int64
		Output   int64
	}
	byModel := map[string]*agg{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var line statsUsageLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			return err
		}
		key := line.Model
		if line.Client != "" {
			key = line.Client + "/" + line.Model
		}
		a := byModel[key]
		if a == nil {
			a = &agg{}
			byModel[key] = a
		}
		a.Requests++
		a.Input += line.InputTokens
		a.Output += line.OutputTokens
	}
	if err := sc.Err(); err != nil {
		return err
	}
	var keys []string
	for k := range byModel {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := byModel[k]
		fmt.Fprintf(out, "%s requests=%d input_tokens=%d output_tokens=%d total_tokens=%d\n",
			k, a.Requests, a.Input, a.Output, a.Input+a.Output)
	}
	return nil
}
