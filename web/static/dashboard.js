(function () {
  const metaEl = document.querySelector('.meta');
  const listEl = document.querySelector('.node-list');
  const stackAnchorEl = document.querySelector('.badge-stack');
  const stackImgEl = document.querySelector('.badge-stack img');
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

  function badgeURL(node) {
    const t = Date.parse(node.collected_at) || 0;
    return '/api/v1/badge/' + encodeURIComponent(node.name) + '.png?t=' + t;
  }

  function stackBadgeURL(collectedAt) {
    const t = Date.parse(collectedAt) || 0;
    return '/api/v1/badge?t=' + t;
  }

  function escapeHTML(s) {
    return String(s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function renderRow(node) {
    const cls = node.reachable ? 'node-row' : 'node-row unreachable';
    const url = badgeURL(node);
    const metricsCls = node.reachable ? 'metrics' : 'metrics error';
    return (
      '<li class="' + cls + '" data-name="' + escapeHTML(node.name) + '">' +
      '<a class="badge" href="' + url + '" title="' + escapeHTML(node.name) + ' badge">' +
      '<img src="' + url + '" alt="' + escapeHTML(node.name) + ' badge" width="128">' +
      '</a>' +
      '<p class="' + metricsCls + '">' + metricsHTML(node) + '</p>' +
      '</li>'
    );
  }

  function setBadgeSrc(img, url) {
    if (img.getAttribute('src') === url) {
      return;
    }
    const pre = new Image();
    pre.onload = function () {
      img.src = url;
    };
    pre.onerror = function () {
      img.src = url;
    };
    pre.src = url;
  }

  function updateRow(li, node) {
    const url = badgeURL(node);
    li.className = node.reachable ? 'node-row' : 'node-row unreachable';
    const anchor = li.querySelector('a.badge');
    const img = li.querySelector('img');
    const metrics = li.querySelector('.metrics');
    if (anchor) {
      anchor.href = url;
      anchor.title = node.name + ' badge';
    }
    if (img) {
      setBadgeSrc(img, url);
      img.alt = node.name + ' badge';
    }
    if (metrics) {
      metrics.className = node.reachable ? 'metrics' : 'metrics error';
      metrics.innerHTML = metricsHTML(node);
    }
  }

  function sameNodeOrder(nodes) {
    const rows = listEl.querySelectorAll('.node-row');
    if (rows.length !== nodes.length) {
      return false;
    }
    for (let i = 0; i < nodes.length; i++) {
      if (rows[i].getAttribute('data-name') !== nodes[i].name) {
        return false;
      }
    }
    return true;
  }

  async function refresh() {
    try {
      const res = await fetch('/api/v1/nodes');
      if (!res.ok) {
        return;
      }
      const data = await res.json();
      metaEl.textContent = 'Updated ' + fmtTime(data.collected_at);
      const stackURL = stackBadgeURL(data.collected_at);
      if (stackAnchorEl) {
        stackAnchorEl.href = stackURL;
      }
      if (stackImgEl) {
        setBadgeSrc(stackImgEl, stackURL);
      }
      const nodes = data.nodes || [];
      if (!sameNodeOrder(nodes)) {
        listEl.innerHTML = nodes.map(renderRow).join('');
        return;
      }
      const rows = listEl.querySelectorAll('.node-row');
      for (let i = 0; i < nodes.length; i++) {
        updateRow(rows[i], nodes[i]);
      }
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
