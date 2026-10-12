// Applies the saved theme before first paint (loaded synchronously in <head>)
// and wires up the toggle button. With no saved choice, the page follows the
// system setting via CSS.
(function () {
    var root = document.documentElement;
    var KEY = 'theme';

    function saved() {
        try {
            var v = localStorage.getItem(KEY);
            return v === 'light' || v === 'dark' ? v : null;
        } catch (e) {
            return null;
        }
    }

    function systemDark() {
        return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
    }

    var initial = saved();
    if (initial) root.setAttribute('data-theme', initial);

    document.addEventListener('DOMContentLoaded', function () {
        var btn = document.getElementById('theme-toggle');
        if (!btn) return;

        function effective() {
            return root.getAttribute('data-theme') || (systemDark() ? 'dark' : 'light');
        }

        function sync() {
            var next = effective() === 'dark' ? 'light' : 'dark';
            btn.setAttribute('aria-label', 'Switch to ' + next + ' mode');
            btn.setAttribute('title', 'Switch to ' + next + ' mode');
        }

        btn.addEventListener('click', function () {
            var next = effective() === 'dark' ? 'light' : 'dark';
            root.setAttribute('data-theme', next);
            try { localStorage.setItem(KEY, next); } catch (e) { /* private mode */ }
            sync();
        });

        sync();
    });
})();
