package flow

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

type FlowDef struct {
	Title string            `yaml:"title"`
	Flows map[string][]Edge `yaml:"flows"`
}

type Edge struct {
	Target string `yaml:"target"`
	Label  string `yaml:"label"`
}

type Graph struct {
	Title string      `json:"title"`
	Nodes []Node      `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type Node struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Missing bool   `json:"missing"`
}

type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label"`
	Origin string `json:"origin"`
}

// ParseFlowYAML parses and validates a flow.yaml file.
func ParseFlowYAML(r io.Reader) (*FlowDef, error) {
	var def FlowDef
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&def); err != nil {
		return nil, fmt.Errorf("invalid flow.yaml: %w", err)
	}
	for src, edges := range def.Flows {
		for i, e := range edges {
			if e.Target == "" {
				return nil, fmt.Errorf("invalid flow.yaml: edge %d from %q has empty target", i, src)
			}
		}
	}
	return &def, nil
}

// ExtractHTMLLinks tokenizes HTML and returns edges for elements with data-dr-link.
func ExtractHTMLLinks(filename string, r io.Reader) ([]Edge, error) {
	tokenizer := html.NewTokenizer(r)
	var edges []Edge
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			if tokenizer.Err() == io.EOF {
				break
			}
			return nil, tokenizer.Err()
		}
		if tt != html.StartTagToken {
			continue
		}
		tn, hasAttr := tokenizer.TagName()
		_ = tn
		if !hasAttr {
			continue
		}
		var target string
		for {
			key, val, more := tokenizer.TagAttr()
			if string(key) == "data-dr-link" {
				target = string(val)
			}
			if !more {
				break
			}
		}
		if target == "" {
			continue
		}
		// Collect text content until the matching end tag.
		label := extractText(tokenizer)
		edges = append(edges, Edge{Target: target, Label: strings.TrimSpace(label)})
	}
	return edges, nil
}

// extractText reads tokens until the next end tag, collecting text.
func extractText(z *html.Tokenizer) string {
	var b strings.Builder
	depth := 1
	for depth > 0 {
		tt := z.Next()
		switch tt {
		case html.TextToken:
			b.Write(z.Text())
		case html.StartTagToken:
			depth++
		case html.EndTagToken:
			depth--
		case html.ErrorToken:
			return b.String()
		}
	}
	return b.String()
}

// BuildGraph merges YAML and HTML edges into a graph with all pages as nodes.
func BuildGraph(pages []string, yamlDef *FlowDef, htmlEdges map[string][]Edge) *Graph {
	pageSet := make(map[string]bool, len(pages))
	for _, p := range pages {
		pageSet[p] = true
	}

	// Dedupe key: source+target → keep YAML over HTML.
	type edgeKey struct{ src, tgt string }
	seen := map[edgeKey]bool{}
	graphEdges := []GraphEdge{}

	// YAML edges first (take precedence).
	if yamlDef != nil {
		for src, edges := range yamlDef.Flows {
			for _, e := range edges {
				k := edgeKey{src, e.Target}
				if seen[k] {
					continue
				}
				seen[k] = true
				graphEdges = append(graphEdges, GraphEdge{Source: src, Target: e.Target, Label: e.Label, Origin: "yaml"})
			}
		}
	}

	// HTML edges (skip duplicates).
	for src, edges := range htmlEdges {
		for _, e := range edges {
			k := edgeKey{src, e.Target}
			if seen[k] {
				continue
			}
			seen[k] = true
			graphEdges = append(graphEdges, GraphEdge{Source: src, Target: e.Target, Label: e.Label, Origin: "html"})
		}
	}

	// Collect all referenced node IDs.
	nodeIDs := make(map[string]bool)
	for _, p := range pages {
		nodeIDs[p] = true
	}
	for _, e := range graphEdges {
		nodeIDs[e.Source] = true
		nodeIDs[e.Target] = true
	}

	sorted := make([]string, 0, len(nodeIDs))
	for id := range nodeIDs {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)

	nodes := make([]Node, len(sorted))
	for i, id := range sorted {
		nodes[i] = Node{ID: id, Label: id, Missing: !pageSet[id]}
	}

	sort.Slice(graphEdges, func(i, j int) bool {
		if graphEdges[i].Source != graphEdges[j].Source {
			return graphEdges[i].Source < graphEdges[j].Source
		}
		return graphEdges[i].Target < graphEdges[j].Target
	})

	title := ""
	if yamlDef != nil {
		title = yamlDef.Title
	}

	return &Graph{Title: title, Nodes: nodes, Edges: graphEdges}
}
