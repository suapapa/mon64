(function () {
  const metaEl = document.querySelector('.meta');
  const gridEl = document.querySelector('.grid');
  if (!metaEl || !gridEl) {
    return;
  }

  function fmtPct(v) {
    return v == null ? null : v.toFixed(1) + '%';
  }

  function fmtTime(iso) {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) {
      return iso;
    }
    return d.toISOString().replace('T', ' ').replace(/\.\d{3}Z$/, ' UTC');
  }

  function metricRow(label, value) {
    const ddClass = value == null ? ' class="na"' : '';
    const text = value == null ? 'n/a' : value;
    return '<div><dt>' + label + '</dt><dd' + ddClass + '>' + text + '</dd></div>';
  }

  function renderCard(node, cacheBust) {
    const cls = node.reachable ? 'card' : 'card unreachable';
    const badgeURL = '/api/v1/badge/' + encodeURIComponent(node.name) + '.png?t=' + cacheBust;
    let body;
    if (!node.reachable) {
      body = '<p class="error">' + escapeHTML(node.last_error || 'unreachable') + '</p>';
    } else {
      const rows = [
        metricRow('CPU', fmtPct(node.cpu)),
        metricRow('GPU', fmtPct(node.gpu)),
        metricRow('MEM', fmtPct(node.mem_used)),
      ];
      if (node.mem_cached != null) {
        rows.push(metricRow('CACHE', fmtPct(node.mem_cached)));
      }
      rows.push(metricRow('SWAP', fmtPct(node.swap_used)));
      body = '<dl>' + rows.join('') + '</dl>';
    }
    return (
      '<article class="' + cls + '">' +
      '<div class="card-head">' +
      '<h2>' + escapeHTML(node.name) + '</h2>' +
      '<a href="' + badgeURL + '" title="badge">' +
      '<img src="' + badgeURL + '" alt="' + escapeHTML(node.name) + ' badge" width="128" height="128">' +
      '</a></div>' + body + '</article>'
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
      gridEl.innerHTML = (data.nodes || []).map(function (n) {
        return renderCard(n, cacheBust);
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
