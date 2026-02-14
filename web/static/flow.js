// Flow Graph Visualization using Cytoscape.js
(function () {
    var cy = null;
    var loaded = false;

    window.initFlowGraph = function (versionID, onNodeClick) {
        var container = document.getElementById("flow-graph");
        if (!container) return;

        // Register dagre layout plugin once
        if (typeof cytoscapeDagre !== "undefined" && !window._dagreRegistered) {
            cytoscapeDagre(cytoscape);
            window._dagreRegistered = true;
        }

        if (loaded && cy) {
            cy.resize();
            cy.fit(undefined, 30);
            return;
        }

        container.innerHTML = '<div style="color:var(--text-muted);padding:2rem;text-align:center">Loading flow graph…</div>';

        fetch("/api/versions/" + versionID + "/flow")
            .then(function (r) { return r.json(); })
            .then(function (data) {
                if (!data.nodes || data.nodes.length === 0) {
                    container.innerHTML = '<div style="color:var(--text-muted);padding:2rem;text-align:center">No pages</div>';
                    return;
                }
                renderGraph(container, data, onNodeClick);
                loaded = true;
            })
            .catch(function () {
                container.innerHTML = '<div style="color:var(--text-muted);padding:2rem;text-align:center">Failed to load flow data</div>';
            });
    };

    window.resetFlowGraph = function () {
        if (cy) { cy.destroy(); cy = null; }
        loaded = false;
    };

    window.setFlowActiveNode = function (pageID) {
        if (!cy) return;
        cy.nodes().removeClass("active");
        cy.getElementById(pageID).addClass("active");
    };

    function renderGraph(container, data, onNodeClick) {
        container.innerHTML = "";

        var elements = [];
        data.nodes.forEach(function (n) {
            elements.push({ data: { id: n.id, label: n.label, missing: n.missing } });
        });
        data.edges.forEach(function (e) {
            elements.push({ data: { source: e.source, target: e.target, label: e.label || "", origin: e.origin } });
        });

        var hasEdges = data.edges && data.edges.length > 0;

        cy = cytoscape({
            container: container,
            elements: elements,
            layout: hasEdges
                ? { name: "dagre", rankDir: "TB", nodeSep: 60, rankSep: 80, padding: 30 }
                : { name: "grid", padding: 30, avoidOverlap: true, condense: true, rows: Math.ceil(Math.sqrt(elements.length)) },
            style: [
                {
                    selector: "node",
                    style: {
                        "label": "data(label)",
                        "text-valign": "center",
                        "text-halign": "center",
                        "background-color": "#27272a",
                        "color": "#fafafa",
                        "border-width": 2,
                        "border-color": "#3f3f46",
                        "shape": "roundrectangle",
                        "width": "label",
                        "height": 36,
                        "padding": "12px",
                        "font-size": 13,
                        "font-family": "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
                        "text-wrap": "none"
                    }
                },
                {
                    selector: "node[?missing]",
                    style: {
                        "border-style": "dashed",
                        "border-color": "#f87171",
                        "border-width": 2
                    }
                },
                {
                    selector: "node.active",
                    style: {
                        "border-color": "#5999fb",
                        "border-width": 3,
                        "shadow-blur": 8,
                        "shadow-color": "#5999fb",
                        "shadow-opacity": 0.4
                    }
                },
                {
                    selector: "edge",
                    style: {
                        "width": 2,
                        "line-color": "#3f3f46",
                        "target-arrow-color": "#3f3f46",
                        "target-arrow-shape": "triangle",
                        "curve-style": "bezier",
                        "label": "data(label)",
                        "font-size": 10,
                        "color": "#a1a1aa",
                        "text-rotation": "autorotate",
                        "text-margin-y": -10
                    }
                },
                {
                    selector: "edge[origin='html']",
                    style: { "line-style": "dashed" }
                }
            ],
            minZoom: 0.3,
            maxZoom: 3
        });

        cy.on("tap", "node", function (evt) {
            var nodeId = evt.target.id();
            if (!evt.target.data("missing") && onNodeClick) {
                onNodeClick(nodeId);
            }
        });

        // Handle container resize
        var ro = new ResizeObserver(function () {
            if (cy) { cy.resize(); cy.fit(undefined, 30); }
        });
        ro.observe(container);
    }
})();
