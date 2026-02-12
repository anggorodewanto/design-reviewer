document.addEventListener("DOMContentLoaded", function () {
    var layout = document.querySelector("[data-version-id]");
    if (!layout) return;

    var projectID = layout.dataset.projectId;
    var currentVersionID = layout.dataset.versionId;
    var frame = document.getElementById("design-frame");
    var tabs = document.getElementById("page-tabs");

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
            if (pages.length > 1) {
                tabs.innerHTML = "";
                pages.forEach(function (p) {
                    var btn = document.createElement("button");
                    btn.className = "page-tab" + (p === defaultPage ? " active" : "");
                    btn.dataset.page = p;
                    btn.textContent = p;
                    tabs.appendChild(btn);
                });
                tabs.style.display = "";
            } else {
                tabs.innerHTML = "";
                tabs.style.display = "none";
            }
        }

        // Update iframe
        frame.src = "/designs/" + versionID + "/" + defaultPage;

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
            tabs.querySelector(".active").classList.remove("active");
            btn.classList.add("active");
            frame.src = "/designs/" + currentVersionID + "/" + btn.dataset.page;
        });
    }
});
