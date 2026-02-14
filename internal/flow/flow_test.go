package flow

import (
	"strings"
	"testing"
)

func TestParseFlowYAML_Valid(t *testing.T) {
	input := `title: "Onboarding"
flows:
  index.html:
    - target: login.html
      label: "Sign In"
    - target: signup.html
      label: "Register"
  login.html:
    - target: dashboard.html
`
	def, err := ParseFlowYAML(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if def.Title != "Onboarding" {
		t.Errorf("title = %q, want Onboarding", def.Title)
	}
	if len(def.Flows["index.html"]) != 2 {
		t.Errorf("index.html edges = %d, want 2", len(def.Flows["index.html"]))
	}
	if def.Flows["login.html"][0].Target != "dashboard.html" {
		t.Errorf("login.html target = %q, want dashboard.html", def.Flows["login.html"][0].Target)
	}
}

func TestParseFlowYAML_EmptyTarget(t *testing.T) {
	input := `flows:
  index.html:
    - target: ""
      label: "Bad"
`
	_, err := ParseFlowYAML(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for empty target")
	}
}

func TestParseFlowYAML_Malformed(t *testing.T) {
	_, err := ParseFlowYAML(strings.NewReader("{{invalid yaml"))
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestParseFlowYAML_UnknownField(t *testing.T) {
	input := `title: "Test"
unknown_field: true
flows: {}
`
	_, err := ParseFlowYAML(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestParseFlowYAML_EmptyFlows(t *testing.T) {
	input := `title: "Empty"
flows: {}
`
	def, err := ParseFlowYAML(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(def.Flows) != 0 {
		t.Errorf("expected empty flows, got %d", len(def.Flows))
	}
}

func TestExtractHTMLLinks_Basic(t *testing.T) {
	html := `<html><body>
<a href="#" data-dr-link="login.html">Sign In</a>
<button data-dr-link="dashboard.html">Continue</button>
<p>No link here</p>
</body></html>`
	edges, err := ExtractHTMLLinks("index.html", strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2", len(edges))
	}
	if edges[0].Target != "login.html" || edges[0].Label != "Sign In" {
		t.Errorf("edge[0] = %+v", edges[0])
	}
	if edges[1].Target != "dashboard.html" || edges[1].Label != "Continue" {
		t.Errorf("edge[1] = %+v", edges[1])
	}
}

func TestExtractHTMLLinks_NoLinks(t *testing.T) {
	html := `<html><body><p>Hello</p></body></html>`
	edges, err := ExtractHTMLLinks("page.html", strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("got %d edges, want 0", len(edges))
	}
}

func TestExtractHTMLLinks_NestedText(t *testing.T) {
	html := `<a data-dr-link="next.html"><span>Go</span> Next</a>`
	edges, err := ExtractHTMLLinks("page.html", strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	if edges[0].Label != "Go Next" {
		t.Errorf("label = %q, want 'Go Next'", edges[0].Label)
	}
}

func TestBuildGraph_MergeWithYAMLPrecedence(t *testing.T) {
	pages := []string{"index.html", "login.html"}
	yamlDef := &FlowDef{
		Title: "Test",
		Flows: map[string][]Edge{
			"index.html": {{Target: "login.html", Label: "YAML Label"}},
		},
	}
	htmlEdges := map[string][]Edge{
		"index.html": {{Target: "login.html", Label: "HTML Label"}},
	}
	g := BuildGraph(pages, yamlDef, htmlEdges)
	if g.Title != "Test" {
		t.Errorf("title = %q", g.Title)
	}
	if len(g.Edges) != 1 {
		t.Fatalf("edges = %d, want 1 (deduped)", len(g.Edges))
	}
	if g.Edges[0].Origin != "yaml" || g.Edges[0].Label != "YAML Label" {
		t.Errorf("edge = %+v, want yaml origin with YAML Label", g.Edges[0])
	}
}

func TestBuildGraph_MissingNodes(t *testing.T) {
	pages := []string{"index.html"}
	yamlDef := &FlowDef{
		Flows: map[string][]Edge{
			"index.html": {{Target: "nonexistent.html", Label: "Go"}},
		},
	}
	g := BuildGraph(pages, yamlDef, nil)
	var missing []string
	for _, n := range g.Nodes {
		if n.Missing {
			missing = append(missing, n.ID)
		}
	}
	if len(missing) != 1 || missing[0] != "nonexistent.html" {
		t.Errorf("missing nodes = %v, want [nonexistent.html]", missing)
	}
}

func TestBuildGraph_DisconnectedNodes(t *testing.T) {
	pages := []string{"a.html", "b.html", "c.html"}
	g := BuildGraph(pages, nil, nil)
	if len(g.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("edges = %d, want 0", len(g.Edges))
	}
	for _, n := range g.Nodes {
		if n.Missing {
			t.Errorf("node %q should not be missing", n.ID)
		}
	}
}

func TestBuildGraph_HTMLOnlyEdges(t *testing.T) {
	pages := []string{"index.html", "about.html"}
	htmlEdges := map[string][]Edge{
		"index.html": {{Target: "about.html", Label: "About"}},
	}
	g := BuildGraph(pages, nil, htmlEdges)
	if g.Title != "" {
		t.Errorf("title = %q, want empty", g.Title)
	}
	if len(g.Edges) != 1 || g.Edges[0].Origin != "html" {
		t.Errorf("edges = %+v", g.Edges)
	}
}

func TestBuildGraph_CircularFlow(t *testing.T) {
	pages := []string{"a.html", "b.html"}
	yamlDef := &FlowDef{
		Flows: map[string][]Edge{
			"a.html": {{Target: "b.html"}},
			"b.html": {{Target: "a.html"}},
		},
	}
	g := BuildGraph(pages, yamlDef, nil)
	if len(g.Edges) != 2 {
		t.Errorf("edges = %d, want 2 (circular)", len(g.Edges))
	}
}

func TestBuildGraph_NodesSorted(t *testing.T) {
	pages := []string{"z.html", "a.html", "m.html"}
	g := BuildGraph(pages, nil, nil)
	for i := 1; i < len(g.Nodes); i++ {
		if g.Nodes[i].ID < g.Nodes[i-1].ID {
			t.Errorf("nodes not sorted: %v", g.Nodes)
			break
		}
	}
}
