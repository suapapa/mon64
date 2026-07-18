(function () {
  const metaEl = document.querySelector('.meta');
  const listEl = document.querySelector('.node-list');
  if (!metaEl || !listEl) {
    return;
  }

  function fmtPct(v) {
    return v == null ? 'n/a' : Math.round(v) + '%';
  }

  function fmtTime(iso) {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) {
      return iso;
    }
    return d.toISOString().replace('T', ' ').replace(/\.\d{3}Z$/, ' UTC');
  }

  function wants(node, kind) {
    return Array.isArray(node.collects) && node.collects.indexOf(kind) !== -1;
  }

  function metricToken(label, value) {
    return label + fmtPct(value);
  }

  function metricsHTML(node) {
    if (!node.reachable) {
      return escapeHTML(node.last_error || 'unreachable');
    }
    const parts = [];
    if (wants(node, 'cpu')) {
      parts.push('<span>' + escapeHTML(metricToken('CPU', node.cpu)) + '</span>');
    }
    if (wants(node, 'gpu')) {
      parts.push('<span>' + escapeHTML(metricToken('GPU', node.gpu)) + '</span>');
    }
    if (wants(node, 'mem')) {
      parts.push('<span>' + escapeHTML(metricToken('MEM', node.mem_used)) + '</span>');
    }
    if (wants(node, 'swap')) {
      parts.push('<span>' + escapeHTML(metricToken('SWAP', node.swap_used)) + '</span>');
    }
    return parts.join('');
  }

  function renderRow(node, cacheBust) {
    const cls = node.reachable ? 'node-row' : 'node-row unreachable';
    const badgeURL = '/api/v1/badge/' + encodeURIComponent(node.name) + '.png?t=' + cacheBust;
    const metricsCls = node.reachable ? 'metrics' : 'metrics error';
    return (
      '<li class="' + cls + '">' +
      '<a class="badge" href="' + badgeURL + '" title="' + escapeHTML(node.name) + ' badge">' +
      '<img src="' + badgeURL + '" alt="' + escapeHTML(node.name) + ' badge" width="128">' +
      '</a>' +
      '<p class="' + metricsCls + '">' + metricsHTML(node) + '</p>' +
      '</li>'
    );
  }

  function escapeHTML(s) {
    return String(s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  async function refresh() {
    try {
      const res = await fetch('/api/v1/nodes');
      if (!res.ok) {
        return;
      }
      const data = await res.json();
      metaEl.textContent = 'Updated ' + fmtTime(data.collected_at);
      const cacheBust = Date.now();
      listEl.innerHTML = (data.nodes || []).map(function (n) {
        return renderRow(n, cacheBust);
      }).join('');
    } catch (_) {
      // ignore transient network errors
    }
  }

  const evtSource = new EventSource('/api/v1/events');
  evtSource.addEventListener('update', refresh);
  evtSource.onerror = function () {
    // EventSource reconnects automatically
  };
})();
