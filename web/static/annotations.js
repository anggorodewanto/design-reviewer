document.addEventListener("DOMContentLoaded", function () {
    var layout = document.querySelector("[data-version-id]");
    if (!layout) return;
    var versionID = layout.dataset.versionId;

    var overlay = document.getElementById("pin-overlay");
    var frame = document.getElementById("design-frame");
    var panel = document.getElementById("comment-panel");
    var filterBtns = document.querySelectorAll(".filter-btn");
    var comments = [];
    var currentFilter = "all";
    var currentPage = "";

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
        fetch("/api/versions/" + versionID + "/comments")
            .then(function (r) { return r.json(); })
            .then(function (data) {
                comments = data || [];
                renderPins();
            });
    }

    // Render pin markers on overlay
    function renderPins() {
        overlay.querySelectorAll(".pin-marker").forEach(function (el) { el.remove(); });
        var num = 0;
        comments.forEach(function (c, i) {
            if (c.page !== currentPage) return;
            if (currentFilter === "open" && c.resolved) return;
            if (currentFilter === "resolved" && !c.resolved) return;
            num++;
            var pin = document.createElement("div");
            pin.className = "pin-marker" + (c.resolved ? " pin-resolved" : "");
            pin.style.left = c.x_percent + "%";
            pin.style.top = c.y_percent + "%";
            pin.textContent = num;
            pin.dataset.index = i;
            pin.addEventListener("click", function (e) {
                e.stopPropagation();
                openPanel(c);
            });
            overlay.appendChild(pin);
        });
    }

    // Click overlay to create new pin
    overlay.addEventListener("click", function (e) {
        var rect = overlay.getBoundingClientRect();
        var xPct = ((e.clientX - rect.left) / rect.width) * 100;
        var yPct = ((e.clientY - rect.top) / rect.height) * 100;
        showNewCommentForm(xPct, yPct);
    });

    function showNewCommentForm(xPct, yPct) {
        panel.innerHTML =
            '<div class="panel-header"><span>New Comment</span><button class="panel-close">&times;</button></div>' +
            '<div class="panel-body">' +
            '<input class="comment-input" placeholder="Your name" id="nc-name">' +
            '<textarea class="comment-input" placeholder="Add a comment..." id="nc-body" rows="3"></textarea>' +
            '<button class="btn-submit" id="nc-submit">Post</button>' +
            '</div>';
        panel.classList.add("open");
        panel.querySelector(".panel-close").onclick = function () { panel.classList.remove("open"); };
        document.getElementById("nc-submit").addEventListener("click", function () {
            var name = document.getElementById("nc-name").value.trim();
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
                panel.classList.remove("open");
                loadComments();
            });
        });
    }

    // Open comment panel for existing pin
    function openPanel(c) {
        var html =
            '<div class="panel-header"><span>Comment</span><button class="panel-close">&times;</button></div>' +
            '<div class="panel-body">' +
            '<div class="comment-item"><strong>' + esc(c.author_name) + '</strong> <span class="comment-time">' + fmtTime(c.created_at) + '</span>' +
            '<p>' + esc(c.body) + '</p></div>';
        if (c.replies) {
            c.replies.forEach(function (r) {
                html += '<div class="reply-item"><strong>' + esc(r.author_name) + '</strong> <span class="comment-time">' + fmtTime(r.created_at) + '</span>' +
                    '<p>' + esc(r.body) + '</p></div>';
            });
        }
        html += '<div class="reply-form">' +
            '<input class="comment-input" placeholder="Your name" id="rp-name">' +
            '<textarea class="comment-input" placeholder="Reply..." id="rp-body" rows="2"></textarea>' +
            '<button class="btn-submit" id="rp-submit">Reply</button>' +
            '</div>' +
            '<button class="btn-resolve" id="rp-resolve">' + (c.resolved ? "Unresolve" : "Resolve") + '</button>' +
            '</div>';
        panel.innerHTML = html;
        panel.classList.add("open");
        panel.querySelector(".panel-close").onclick = function () { panel.classList.remove("open"); };
        document.getElementById("rp-submit").addEventListener("click", function () {
            var name = document.getElementById("rp-name").value.trim();
            var body = document.getElementById("rp-body").value.trim();
            if (!body) return;
            fetch("/api/comments/" + c.id + "/replies", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ author_name: name || "Anonymous", author_email: "", body: body })
            }).then(function () { loadComments(); openPanelById(c.id); });
        });
        document.getElementById("rp-resolve").addEventListener("click", function () {
            fetch("/api/comments/" + c.id + "/resolve", { method: "PATCH" })
                .then(function () { loadComments(); panel.classList.remove("open"); });
        });
    }

    function openPanelById(id) {
        var c = comments.find(function (x) { return x.id === id; });
        if (c) openPanel(c);
    }

    function esc(s) {
        var d = document.createElement("div");
        d.textContent = s || "";
        return d.innerHTML;
    }

    function fmtTime(iso) {
        if (!iso) return "";
        var d = new Date(iso);
        return d.toLocaleDateString() + " " + d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    }

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
        panel.classList.remove("open");
        loadComments();
    };

    loadComments();
});
