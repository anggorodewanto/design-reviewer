document.addEventListener("DOMContentLoaded", function () {
    var layout = document.querySelector("[data-version-id]");
    if (!layout) return;
    var versionID = layout.dataset.versionId;

    var overlay = document.getElementById("pin-overlay");
    var frame = document.getElementById("design-frame");
    var panel = document.getElementById("comment-panel");
    var panelBackdrop = document.getElementById("comment-panel-backdrop");
    var filterBtns = document.querySelectorAll(".filter-btn");
    var comments = [];
    var currentFilter = "all";
    var currentPage = "";
    var savedPanelPosition = null;
    var shortcutHint = /Mac|iPhone|iPad/.test(navigator.platform) ? "⌘+Enter to post" : "Ctrl+Enter to post";

    // Close panel when clicking on backdrop
    panelBackdrop.addEventListener("click", function(e) {
        if (e.target === panelBackdrop) {
            panelBackdrop.classList.remove("open");
            savedPanelPosition = null;
        }
    });

    // Determine current page from iframe src
    function getCurrentPage() {
        try {
            var src = frame.src;
            return src.substring(src.lastIndexOf("/") + 1);
        } catch (e) {
            return "";
        }
    }

    currentPage = getCurrentPage();

    // Load comments from API
    function loadComments() {
        return fetch("/api/versions/" + versionID + "/comments")
            .then(function (r) { return r.json(); })
            .then(function (data) {
                comments = data || [];
                renderPins();
            });
    }

    // Render pin markers on overlay
    function renderPins() {
        overlay.querySelectorAll(".pin-marker").forEach(function (el) { el.remove(); });
        var pageComments = [];
        comments.forEach(function (c) { if (c.page === currentPage) pageComments.push(c); });
        pageComments.forEach(function (c, i) {
            var num = i + 1;
            if (currentFilter === "open" && c.resolved) return;
            if (currentFilter === "resolved" && !c.resolved) return;
            var pin = document.createElement("div");
            pin.className = "pin-marker" + (c.resolved ? " pin-resolved" : "");
            pin.style.left = c.x_percent + "%";
            pin.style.top = c.y_percent + "%";
            pin.textContent = num;
            pin.dataset.index = i;
            pin.addEventListener("click", function (e) {
                e.stopPropagation();
                if (pin.dataset.dragged) { delete pin.dataset.dragged; return; }
                openPanel(c, pin);
            });
            pin.addEventListener("mousedown", function (e) {
                e.stopPropagation();
                e.preventDefault();
                var startX = e.clientX, startY = e.clientY;
                var dragging = false;
                function onMove(ev) {
                    var dx = ev.clientX - startX, dy = ev.clientY - startY;
                    if (!dragging && dx * dx + dy * dy < 16) return;
                    if (!dragging) { dragging = true; pin.classList.add("pin-dragging"); }
                    var rect = overlay.getBoundingClientRect();
                    var x = Math.max(0, Math.min(100, ((ev.clientX - rect.left) / rect.width) * 100));
                    var y = Math.max(0, Math.min(100, ((ev.clientY - rect.top) / rect.height) * 100));
                    pin.style.left = x + "%";
                    pin.style.top = y + "%";
                }
                function onUp(ev) {
                    document.removeEventListener("mousemove", onMove);
                    document.removeEventListener("mouseup", onUp);
                    pin.classList.remove("pin-dragging");
                    if (!dragging) return;
                    pin.dataset.dragged = "1";
                    var rect = overlay.getBoundingClientRect();
                    var nx = Math.max(0, Math.min(100, ((ev.clientX - rect.left) / rect.width) * 100));
                    var ny = Math.max(0, Math.min(100, ((ev.clientY - rect.top) / rect.height) * 100));
                    fetch("/api/comments/" + c.id + "/move", {
                        method: "PATCH",
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({ x_percent: nx, y_percent: ny })
                    }).then(function () { loadComments(); });
                }
                document.addEventListener("mousemove", onMove);
                document.addEventListener("mouseup", onUp);
            });
            overlay.appendChild(pin);
        });
    }

    // Click overlay to create new pin
    overlay.addEventListener("click", function (e) {
        var rect = overlay.getBoundingClientRect();
        var xPct = ((e.clientX - rect.left) / rect.width) * 100;
        var yPct = ((e.clientY - rect.top) / rect.height) * 100;
        showNewCommentForm(xPct, yPct, e.clientX, e.clientY);
    });

    function positionPanel(clientX, clientY) {
        var panelWidth = 360;
        var panelHeight = 800;
        var spacing = 12;
        var left = clientX + spacing;
        if (left + panelWidth > window.innerWidth - 20) left = clientX - panelWidth - spacing;
        var top = clientY;
        if (top + panelHeight > window.innerHeight - 20) top = window.innerHeight - panelHeight - 20;
        if (top < 20) top = 20;
        panel.style.left = left + 'px';
        panel.style.top = top + 'px';
        savedPanelPosition = { left: left, top: top };
    }

    function showNewCommentForm(xPct, yPct, clientX, clientY) {
        var nameField = window.authUser ? '' : '<input class="comment-input" placeholder="Your name" id="nc-name">';
        panel.innerHTML =
            '<div class="panel-header"><span>New Comment</span><button class="panel-close">&times;</button></div>' +
            '<div class="panel-body">' +
            nameField +
            '<textarea class="comment-input" placeholder="Add a comment..." id="nc-body" rows="3"></textarea>' +
            '<span class="shortcut-hint">' + shortcutHint + '</span>' +
            '<button class="btn-submit" id="nc-submit">Post</button>' +
            '</div>';
        positionPanel(clientX, clientY);
        panelBackdrop.classList.add("open");
        // Auto-focus the comment input
        setTimeout(function() {
            var textarea = document.getElementById("nc-body");
            if (textarea) textarea.focus();
        }, 100);
        panel.querySelector(".panel-close").onclick = function () {
            panelBackdrop.classList.remove("open");
            savedPanelPosition = null;
        };
        document.getElementById("nc-submit").addEventListener("click", function () {
            var nameEl = document.getElementById("nc-name");
            var name = window.authUser ? window.authUser.name : (nameEl ? nameEl.value.trim() : "Anonymous");
            var body = document.getElementById("nc-body").value.trim();
            if (!body) return;
            fetch("/api/versions/" + versionID + "/comments", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    page: currentPage,
                    x_percent: xPct,
                    y_percent: yPct,
                    author_name: name || "Anonymous",
                    author_email: "",
                    body: body
                })
            }).then(function () {
                panelBackdrop.classList.remove("open");
                loadComments();
            });
        });
        document.getElementById("nc-body").addEventListener("keydown", function (e) {
            if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
                e.preventDefault();
                document.getElementById("nc-submit").click();
            }
        });
    }

    // Open comment panel for existing pin
    function openPanel(c, sourceElement) {
        var resolveBtn = c.resolved
            ? '<button class="btn-resolve-header" id="rp-resolve"><svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="currentColor" viewBox="0 0 256 256"><path d="M173.66,98.34a8,8,0,0,1,0,11.32l-56,56a8,8,0,0,1-11.32,0l-24-24a8,8,0,0,1,11.32-11.32L112,148.69l50.34-50.35A8,8,0,0,1,173.66,98.34ZM232,128A104,104,0,1,1,128,24,104.11,104.11,0,0,1,232,128Zm-16,0a88,88,0,1,0-88,88A88.1,88.1,0,0,0,216,128Z"></path></svg>Unresolve</button>'
            : '<button class="btn-resolve-header" id="rp-resolve"><svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="currentColor" viewBox="0 0 256 256"><path d="M173.66,98.34a8,8,0,0,1,0,11.32l-56,56a8,8,0,0,1-11.32,0l-24-24a8,8,0,0,1,11.32-11.32L112,148.69l50.34-50.35A8,8,0,0,1,173.66,98.34ZM232,128A104,104,0,1,1,128,24,104.11,104.11,0,0,1,232,128Zm-16,0a88,88,0,1,0-88,88A88.1,88.1,0,0,0,216,128Z"></path></svg>Resolve</button>';

        var commentsHtml = '<div class="comment-item"><strong class="comment-author">' + esc(c.author_name) + '</strong> <span class="comment-time">' + fmtTime(c.created_at) + '</span>' +
            '<p class="comment-body">' + esc(c.body) + '</p></div>';
        if (c.replies) {
            c.replies.forEach(function (r) {
                commentsHtml += '<div class="reply-item"><strong class="comment-author">' + esc(r.author_name) + '</strong> <span class="comment-time">' + fmtTime(r.created_at) + '</span>' +
                    '<p class="comment-body">' + esc(r.body) + '</p></div>';
            });
        }

        var html =
            '<div class="panel-header"><span>Comment</span><div class="panel-actions">' + resolveBtn + '<button class="panel-close">&times;</button></div></div>' +
            '<div class="panel-body">' +
            '<div class="comments-scroll">' + commentsHtml + '</div>' +
            '<div class="reply-form">' +
            (window.authUser ? '' : '<input class="comment-input" placeholder="Your name" id="rp-name">') +
            '<textarea class="comment-input" placeholder="Reply..." id="rp-body" rows="2"></textarea>' +
            '<span class="shortcut-hint">' + shortcutHint + '</span>' +
            '<button class="btn-submit" id="rp-submit">Reply</button>' +
            '</div></div>';
        panel.innerHTML = html;

        // Position the panel next to the source element or use saved position
        if (savedPanelPosition) {
            panel.style.left = savedPanelPosition.left + 'px';
            panel.style.top = savedPanelPosition.top + 'px';
        } else if (sourceElement) {
            var rect = sourceElement.getBoundingClientRect();
            positionPanel(rect.right, rect.top);
        }

        panelBackdrop.classList.add("open");
        panel.querySelector(".panel-close").onclick = function () {
            panelBackdrop.classList.remove("open");
            savedPanelPosition = null;
        };
        document.getElementById("rp-submit").addEventListener("click", function () {
            var nameEl = document.getElementById("rp-name");
            var name = window.authUser ? window.authUser.name : (nameEl ? nameEl.value.trim() : "Anonymous");
            var body = document.getElementById("rp-body").value.trim();
            if (!body) return;
            fetch("/api/comments/" + c.id + "/replies", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ author_name: name || "Anonymous", author_email: "", body: body })
            }).then(function () { loadComments().then(function () { openPanelById(c.id); }); });
        });
        document.getElementById("rp-body").addEventListener("keydown", function (e) {
            if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
                e.preventDefault();
                document.getElementById("rp-submit").click();
            }
        });
        document.getElementById("rp-resolve").addEventListener("click", function () {
            fetch("/api/comments/" + c.id + "/resolve", { method: "PATCH" })
                .then(function () {
                    loadComments();
                    panelBackdrop.classList.remove("open");
                    savedPanelPosition = null;
                });
        });
    }

    function openPanelById(id) {
        var c = comments.find(function (x) { return x.id === id; });
        if (c) openPanel(c);
    }

    function esc(s) {
        var d = document.createElement("div");
        d.textContent = s || "";
        return d.innerHTML.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    }

    function fmtTime(iso) {
        if (!iso) return "";
        var d = new Date(iso);
        return d.toLocaleDateString() + " " + d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    }

    // --- Comments Sidebar ---
    var csSidebar = document.getElementById("comments-sidebar");
    var csList = document.getElementById("comments-sidebar-list");


    function renderCommentsSidebar() {
        if (!csList) return;
        csList.innerHTML = "";
        var pageComments = [];
        comments.forEach(function (c) { if (c.page === currentPage) pageComments.push(c); });
        var shown = 0;
        pageComments.forEach(function (c, i) {
            var num = i + 1;
            if (currentFilter === "open" && c.resolved) return;
            if (currentFilter === "resolved" && !c.resolved) return;
            shown++;
            var item = document.createElement("div");
            item.className = "cs-item" + (c.resolved ? " cs-resolved" : "");
            item.innerHTML = '<div style="display:flex;align-items:center;gap:6px"><span class="cs-pin-num">' + num + '</span><span class="cs-author">' + esc(c.author_name) + '</span></div>' +
                '<div class="cs-body">' + esc(c.body) + '</div>' +
                '<div class="cs-meta"><span>' + fmtTime(c.created_at) + '</span>' +
                (c.replies && c.replies.length ? '<span>' + c.replies.length + ' repl' + (c.replies.length === 1 ? 'y' : 'ies') + '</span>' : '') +
                '</div>';
            item.addEventListener("click", function (e) {
                scrollToPin(c);
                openPanel(c, item);
            });
            csList.appendChild(item);
        });
        if (shown === 0) csList.innerHTML = '<div style="padding:1rem;text-align:center;color:var(--text-muted);font-size:0.8rem;">No comments</div>';
    }

    function scrollToPin(c) {
        var wrapper = document.querySelector(".iframe-wrapper");
        if (!wrapper) return;
        var pinY = (c.y_percent / 100) * overlay.offsetHeight;
        wrapper.scrollTo({ top: Math.max(0, pinY - wrapper.clientHeight / 3), behavior: "smooth" });
    }

    // Patch renderPins to also update sidebar
    var _origRenderPins = renderPins;
    renderPins = function () {
        _origRenderPins();
        if (csSidebar && csSidebar.classList.contains("open")) renderCommentsSidebar();
    };

    // Viewport switcher
    document.querySelectorAll(".viewport-btn").forEach(function (btn) {
        btn.addEventListener("click", function () {
            document.querySelectorAll(".viewport-btn").forEach(function (b) { b.classList.remove("active"); });
            btn.classList.add("active");
            var w = btn.dataset.width + "px";
            frame.style.width = w;
            frame.style.minWidth = w;
            setTimeout(function () { window.dispatchEvent(new Event("resize")); }, 50);
        });
    });

    // Filter buttons
    filterBtns.forEach(function (btn) {
        btn.addEventListener("click", function () {
            filterBtns.forEach(function (b) { b.classList.remove("active"); });
            btn.classList.add("active");
            currentFilter = btn.dataset.filter;
            renderPins();
        });
    });

    // Page-aware: update pins when page tab changes
    var tabs = document.getElementById("page-tabs");
    if (tabs) {
        tabs.addEventListener("click", function (e) {
            var btn = e.target.closest(".page-tab");
            if (!btn) return;
            setTimeout(function () {
                currentPage = getCurrentPage();
                renderPins();
            }, 50);
        });
    }

    frame.addEventListener("load", function () {
        currentPage = getCurrentPage();
        renderPins();
    });

    // Expose reload hook for version switching
    window.reloadComments = function (newVersionID) {
        versionID = newVersionID;
        currentPage = getCurrentPage();
        panelBackdrop.classList.remove("open");
        loadComments();
    };

    loadComments();
});
