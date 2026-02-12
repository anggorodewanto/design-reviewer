document.addEventListener("DOMContentLoaded", function () {
    var tabs = document.getElementById("page-tabs");
    if (!tabs) return;
    var frame = document.getElementById("design-frame");
    var base = frame.src.replace(/\/[^/]*$/, "/");

    tabs.addEventListener("click", function (e) {
        var btn = e.target.closest(".page-tab");
        if (!btn) return;
        tabs.querySelector(".active").classList.remove("active");
        btn.classList.add("active");
        frame.src = base + btn.dataset.page;
    });
});
