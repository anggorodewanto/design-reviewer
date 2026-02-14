(function() {
    const shareBtn = document.getElementById('share-btn');
    if (!shareBtn) return;

    const dialog = document.getElementById('share-dialog');
    const generateBtn = document.getElementById('generate-invite');
    const linkBox = document.getElementById('invite-link-box');
    const linkInput = document.getElementById('invite-link');
    const copyBtn = document.getElementById('copy-invite');
    const closeBtn = document.getElementById('close-share');
    const membersList = document.getElementById('members-list');
    const projectID = document.querySelector('.viewer-layout').dataset.projectId;

    shareBtn.addEventListener('click', function() {
        dialog.style.display = 'flex';
        loadMembers();
    });

    closeBtn.addEventListener('click', function() {
        dialog.style.display = 'none';
    });

    generateBtn.addEventListener('click', function() {
        fetch('/api/projects/' + projectID + '/invites', { method: 'POST' })
            .then(r => r.json())
            .then(data => {
                linkInput.value = data.invite_url;
                linkBox.style.display = 'flex';
            });
    });

    copyBtn.addEventListener('click', function() {
        linkInput.select();
        navigator.clipboard.writeText(linkInput.value);
        copyBtn.textContent = 'Copied!';
        setTimeout(() => { copyBtn.textContent = 'Copy'; }, 2000);
    });

    function esc(s) {
        var d = document.createElement('div');
        d.textContent = s || '';
        return d.innerHTML;
    }

    function loadMembers() {
        fetch('/api/projects/' + projectID + '/members')
            .then(r => r.json())
            .then(members => {
                if (!members || members.length === 0) {
                    membersList.innerHTML = '<p class="empty">No members yet</p>';
                    return;
                }
                membersList.innerHTML = members.map(m =>
                    '<div class="member-row"><span>' + esc(m.email) + '</span>' +
                    (window.isOwner ? '<button class="btn-remove" data-email="' + esc(m.email) + '">Remove</button>' : '') +
                    '</div>'
                ).join('');
                membersList.querySelectorAll('.btn-remove').forEach(btn => {
                    btn.addEventListener('click', function() {
                        fetch('/api/projects/' + projectID + '/members/' + encodeURIComponent(this.dataset.email), { method: 'DELETE' })
                            .then(() => loadMembers());
                    });
                });
            });
    }
})();
