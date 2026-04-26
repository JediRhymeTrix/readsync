/* ReadSync admin UI - minimal HTMX-compatible shim.
 *
 * Supported attributes:
 *   hx-post       URL to POST to
 *   hx-target     CSS selector to write the response into
 *   hx-swap       innerHTML | outerHTML | none (default innerHTML)
 *   hx-headers    JSON object of additional headers (e.g. CSRF token)
 *   hx-confirm    confirm() prompt before issuing the request
 *
 * For simple use cases this is a 1:1 drop-in replacement for HTMX.
 * For richer behaviour, swap this file for the real HTMX library.
 */
(function () {
    'use strict';

    function csrfToken() {
        var m = document.querySelector('meta[name="csrf-token"]');
        return m ? m.getAttribute('content') : '';
    }

    function getHeaders(el) {
        var h = { 'X-Requested-With': 'XMLHttpRequest' };
        var raw = el.getAttribute('hx-headers');
        if (raw) {
            try { Object.assign(h, JSON.parse(raw)); } catch (e) { /* ignore */ }
        }
        // Fallback: always attach CSRF if a meta token exists.
        if (!h['X-ReadSync-CSRF']) {
            var t = csrfToken();
            if (t) h['X-ReadSync-CSRF'] = t;
        }
        return h;
    }

    function findTarget(el) {
        var sel = el.getAttribute('hx-target');
        if (!sel) return el;
        var t = document.querySelector(sel);
        return t || el;
    }

    function applySwap(target, swap, content) {
        if (swap === 'none') return;
        if (swap === 'outerHTML') {
            var div = document.createElement('div');
            div.innerHTML = content;
            target.replaceWith.apply(target, div.children);
            return;
        }
        target.innerHTML = content;
        // Re-bind handlers on inserted content.
        bind(target);
    }

    function buildBody(el) {
        // If a form, serialise as form-urlencoded.
        if (el.tagName === 'FORM') {
            return new FormData(el);
        }
        // If the element is inside a form, serialise that.
        var form = el.closest('form');
        if (form) return new FormData(form);
        return null;
    }

    function send(el, ev) {
        var url = el.getAttribute('hx-post');
        if (!url) return;
        if (el.tagName === 'FORM' || el.tagName === 'BUTTON') {
            ev.preventDefault();
        }
        var confirmMsg = el.getAttribute('hx-confirm');
        if (confirmMsg && !window.confirm(confirmMsg)) return;
        var swap = el.getAttribute('hx-swap') || 'innerHTML';
        var target = findTarget(el);
        var headers = getHeaders(el);
        var body = buildBody(el);

        fetch(url, {
            method: 'POST',
            headers: headers,
            body: body,
            credentials: 'same-origin'
        }).then(function (r) {
            return r.text().then(function (txt) { return { status: r.status, body: txt }; });
        }).then(function (resp) {
            if (resp.status >= 400) {
                applySwap(target, swap,
                    '<div class="rs-error">Error ' + resp.status + ': ' +
                    escapeHTML(resp.body) + '</div>');
                return;
            }
            applySwap(target, swap, resp.body);
        }).catch(function (e) {
            applySwap(target, swap, '<div class="rs-error">Network error: ' + escapeHTML(String(e)) + '</div>');
        });
    }

    function escapeHTML(s) {
        return String(s).replace(/[&<>"']/g, function (c) {
            return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c];
        });
    }

    function bind(root) {
        var nodes = (root || document).querySelectorAll('[hx-post]');
        nodes.forEach(function (el) {
            if (el.dataset.rsBound) return;
            el.dataset.rsBound = '1';
            var ev = (el.tagName === 'FORM') ? 'submit' : 'click';
            el.addEventListener(ev, function (e) { send(el, e); });
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function () { bind(); });
    } else {
        bind();
    }

    // Expose for tests / advanced usage.
    window.RS = { bind: bind };
})();
