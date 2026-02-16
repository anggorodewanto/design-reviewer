document.addEventListener("DOMContentLoaded", function () {
    var layout = document.querySelector("[data-version-id]");
    if (!layout) return;

    var projectID = layout.dataset.projectId;
    var currentVersionID = layout.dataset.versionId;
    var frame = document.getElementById("design-frame");
    var tabs = document.getElementById("page-tabs");

    // Auto-resize iframe and pin overlay to content dimensions
    function resizeFrame() {
        try {
            var doc = frame.contentDocument || frame.contentWindow.document;
            var body = doc.body;
            var html = doc.documentElement;

            // Inject a temporary style to override fixed heights
            var styleId = 'dr-height-override';
            var existingStyle = doc.getElementById(styleId);
            if (existingStyle) existingStyle.remove();

            var style = doc.createElement('style');
            style.id = styleId;
            style.textContent = 'html, body, * { height: auto !important; min-height: auto !important; }';
            doc.head.appendChild(style);

            // Force reflow
            void(body.offsetHeight);

            // Measure the true content height
            var h = Math.max(
                body.scrollHeight, body.offsetHeight,
                html.scrollHeight, html.offsetHeight
            );

            var w = Math.max(
                body.scrollWidth, body.offsetWidth,
                html.scrollWidth, html.offsetWidth
            );

            // Remove the override style
            style.remove();

            // Apply the measured height to iframe
            if (h > 0) {
                frame.style.height = h + "px";
            }

            var overlay = document.getElementById("pin-overlay");
            if (overlay) {
                overlay.style.height = h + "px";
                overlay.style.width = w + "px";
            }
        } catch (e) {
            console.error('resizeFrame error:', e);
        }
    }
    frame.addEventListener("load", resizeFrame);
    frame.addEventListener("load", function () {
        try {
            var doc = frame.contentDocument || frame.contentWindow.document;
            var overlay = document.getElementById("pin-overlay");
            if (!overlay) return;

            var tooltip = document.createElement("div");
            tooltip.className = "dr-link-tooltip";
            overlay.appendChild(tooltip);

            overlay.addEventListener("mousemove", function (e) {
                var iframeRect = frame.getBoundingClientRect();
                try { var el = doc.elementFromPoint(e.clientX - iframeRect.left, e.clientY - iframeRect.top); } catch(ex) { return; }
                var link = el && el.closest("[data-dr-link]");
                if (link) {
                    tooltip.textContent = (navigator.platform.indexOf("Mac") > -1 ? "⌘" : "Ctrl") + "+Click → " + link.getAttribute("data-dr-link");
                    tooltip.style.left = (e.clientX - overlay.getBoundingClientRect().left + 16) + "px";
                    tooltip.style.top = (e.clientY - overlay.getBoundingClientRect().top - 10) + "px";
                    tooltip.style.display = "block";
                } else {
                    tooltip.style.display = "none";
                }
            });
            overlay.addEventListener("mouseleave", function () {
                tooltip.style.display = "none";
            });

            overlay.addEventListener("click", function (e) {
                if (!e.ctrlKey && !e.metaKey) return;
                var iframeRect = frame.getBoundingClientRect();
                var x = e.clientX - iframeRect.left;
                var y = e.clientY - iframeRect.top;
                var el = doc.elementFromPoint(x, y);
                if (!el) return;
                var link = el.closest("[data-dr-link]");
                if (!link) return;
                e.preventDefault();
                e.stopPropagation();
                var target = link.getAttribute("data-dr-link");
                var tab = tabs && tabs.querySelector('[data-page="' + target + '"]');
                if (tab) tab.click();
            });
        } catch (e) {}
    });
    window.addEventListener("resize", resizeFrame);

    // Fetch and render version list in sidebar
    fetch("/api/projects/" + projectID + "/versions")
        .then(function (r) { return r.json(); })
        .then(function (versions) {
            var list = document.getElementById("version-list");
            if (!list) return;
            list.innerHTML = "";
            versions.forEach(function (v) {
                var item = document.createElement("div");
                item.className = "version-item" + (v.id === currentVersionID ? " active" : "");
                item.textContent = "v" + v.version_num + " — " + new Date(v.created_at).toLocaleDateString();
                item.dataset.versionId = v.id;
                item.dataset.pages = JSON.stringify(v.pages || []);
                item.addEventListener("click", function () {
                    switchVersion(v.id, v.pages || []);
                });
                list.appendChild(item);
            });
        });

    function switchVersion(versionID, pages) {
        if (versionID === currentVersionID) return;
        currentVersionID = versionID;
        layout.dataset.versionId = versionID;

        // Update sidebar highlight
        document.querySelectorAll(".version-item").forEach(function (el) {
            el.classList.toggle("active", el.dataset.versionId === versionID);
        });

        // Update page tabs
        var defaultPage = pages.indexOf("index.html") >= 0 ? "index.html" : (pages[0] || "");
        if (tabs) {
            tabs.innerHTML = "";
            var flowBtn = document.createElement("button");
            flowBtn.className = "page-tab";
            flowBtn.dataset.page = "__flow__";
            flowBtn.textContent = "Flow";
            tabs.appendChild(flowBtn);
            pages.forEach(function (p) {
                var btn = document.createElement("button");
                btn.className = "page-tab" + (p === defaultPage ? " active" : "");
                btn.dataset.page = p;
                btn.textContent = p;
                tabs.appendChild(btn);
            });
        }

        // Show iframe, hide flow graph
        var flowContainer = document.getElementById("flow-graph");
        var iframeWrapper = document.querySelector(".iframe-wrapper");
        if (flowContainer) flowContainer.style.display = "none";
        if (iframeWrapper) iframeWrapper.style.display = "";

        // Update iframe
        frame.src = "/designs/" + versionID + "/" + defaultPage;
        frame.parentElement.scrollTop = 0;

        // Reset flow graph for new version
        if (window.resetFlowGraph) window.resetFlowGraph();

        // Update URL
        history.replaceState(null, "", "/projects/" + projectID + "?version=" + versionID);

        // Reload comments for new version
        if (window.reloadComments) {
            window.reloadComments(versionID);
        }
    }

    // Page tab switching
    if (tabs) {
        tabs.addEventListener("click", function (e) {
            var btn = e.target.closest(".page-tab");
            if (!btn) return;
            var active = tabs.querySelector(".active");
            if (active) active.classList.remove("active");
            btn.classList.add("active");

            var page = btn.dataset.page;
            var flowContainer = document.getElementById("flow-graph");
            var iframeWrapper = document.querySelector(".iframe-wrapper");

            if (page === "__flow__") {
                if (iframeWrapper) iframeWrapper.style.display = "none";
                if (flowContainer) flowContainer.style.display = "block";
                if (window.initFlowGraph) {
                    window.initFlowGraph(currentVersionID, function (nodeId) {
                        // Click node → switch to that page tab
                        var pageBtn = tabs.querySelector('[data-page="' + nodeId + '"]');
                        if (pageBtn) pageBtn.click();
                    });
                }
            } else {
                if (flowContainer) flowContainer.style.display = "none";
                if (iframeWrapper) iframeWrapper.style.display = "";
                frame.src = "/designs/" + currentVersionID + "/" + page;
                frame.parentElement.scrollTop = 0;
                if (window.setFlowActiveNode) window.setFlowActiveNode(page);
            }
        });
    }

    // Status change
    var statusSelect = document.getElementById("status-select");
    if (statusSelect) {
        statusSelect.addEventListener("change", function () {
            var status = statusSelect.value;
            fetch("/api/projects/" + projectID + "/status", {
                method: "PATCH",
                headers: {"Content-Type": "application/json"},
                body: JSON.stringify({status: status})
            }).then(function (r) {
                if (!r.ok) {
                    statusSelect.value = statusSelect.dataset.prev;
                    return;
                }
                statusSelect.className = "status-select badge badge-" + status;
                statusSelect.dataset.prev = status;
            });
        });
        statusSelect.dataset.prev = statusSelect.value;
    }

    // Mode switching (View Mode vs Comment Mode)
    var modeBtns = document.querySelectorAll(".mode-btn-floating");
    var pinOverlay = document.getElementById("pin-overlay");
    var versionsPanel = document.getElementById("versions-panel");
    var commentsPanel = document.getElementById("comments-panel-sidebar");
    var currentMode = "view"; // Default to view mode

    function switchMode(mode) {
        if (mode === currentMode) return;
        currentMode = mode;

        // Update active button
        modeBtns.forEach(function (b) {
            b.classList.toggle("active", b.dataset.mode === mode);
        });

        // Toggle comment-mode class on layout for cursor control
        if (mode === "comment") {
            layout.classList.add("comment-mode");
        } else {
            layout.classList.remove("comment-mode");
        }

        // Toggle pin overlay based on mode
        if (pinOverlay) {
            if (mode === "view") {
                pinOverlay.classList.add("view-mode");
            } else {
                pinOverlay.classList.remove("view-mode");
            }
        }

        // Switch sidebar panels
        if (mode === "view") {
            if (versionsPanel) versionsPanel.classList.add("active");
            if (commentsPanel) commentsPanel.classList.remove("active");
        } else {
            if (versionsPanel) versionsPanel.classList.remove("active");
            if (commentsPanel) commentsPanel.classList.add("active");
            // Render comments sidebar when switching to comment mode
            if (window.renderCommentsSidebar) {
                window.renderCommentsSidebar();
            }
        }
    }

    // Initialize with view mode
    switchMode("view");

    // Mode button clicks
    modeBtns.forEach(function (btn) {
        btn.addEventListener("click", function () {
            switchMode(btn.dataset.mode);
        });
    });

    // Keyboard shortcuts: V for View, C for Comment
    document.addEventListener("keydown", function (e) {
        // Ignore if user is typing in an input/textarea in the main document
        if (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA") return;

        // Ignore if user is currently focused inside the iframe
        if (document.activeElement === frame) return;

        if (e.key.toLowerCase() === "v") {
            e.preventDefault();
            switchMode("view");
        } else if (e.key.toLowerCase() === "c") {
            e.preventDefault();
            switchMode("comment");
        }
    });

    // Also listen for keyboard shortcuts from inside the iframe
    frame.addEventListener("load", function() {
        try {
            var iframeDoc = frame.contentDocument || frame.contentWindow.document;
            iframeDoc.addEventListener("keydown", function(e) {
                // Only allow shortcuts if user is NOT typing in an input/textarea inside iframe
                if (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA") return;

                if (e.key.toLowerCase() === "v") {
                    e.preventDefault();
                    switchMode("view");
                } else if (e.key.toLowerCase() === "c") {
                    e.preventDefault();
                    switchMode("comment");
                }
            });
        } catch (e) {
            // Cross-origin iframe, can't attach event listener
        }
    });
});
