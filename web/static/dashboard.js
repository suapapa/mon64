(function () {
  const metaEl = document.querySelector('.meta');
  const fleetEl = document.getElementById('fleet');
  const stackAnchorEl = document.querySelector('.badge-stack');
  const stackImgEl = document.querySelector('.badge-stack img');
  const liveEl = document.getElementById('live-status');
  const liveLabelEl = liveEl && liveEl.querySelector('.live-label');

  if (!metaEl) {
    return;
  }

  const evtSource = new EventSource('/api/v1/events');

  function setLive(state, label) {
    if (!liveEl || !liveLabelEl) {
      return;
    }
    liveEl.dataset.state = state;
    liveLabelEl.textContent = label;
  }

  function syncLiveFromFeed() {
    if (evtSource.readyState === EventSource.OPEN) {
      setLive('live', 'live');
    } else if (evtSource.readyState === EventSource.CONNECTING) {
      setLive('connecting', 'connecting');
    } else {
      setLive('reconnecting', 'reconnecting');
    }
  }

  function fmtPct(v) {
    return v == null ? 'n/a' : Math.round(v) + '%';
  }

  function loadLevel(v) {
    if (v == null || Number.isNaN(v)) {
      return 'na';
    }
    if (v <= 33) {
      return 'low';
    }
    if (v <= 66) {
      return 'mid';
    }
    if (v < 90) {
      return 'high';
    }
    return 'crit';
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

  function metricHTML(label, value) {
    return (
      '<span class="metric" data-level="' + loadLevel(value) + '">' +
      '<span class="metric-label">' + escapeHTML(label) + '</span>' +
      '<span class="metric-value">' + escapeHTML(fmtPct(value)) + '</span>' +
      '</span>'
    );
  }

  function metricsHTML(node) {
    if (!node.reachable) {
      return escapeHTML(node.last_error || 'unreachable');
    }
    const parts = [];
    if (wants(node, 'cpu')) {
      parts.push(metricHTML('CPU', node.cpu));
    }
    if (wants(node, 'gpu')) {
      parts.push(metricHTML('GPU', node.gpu));
    }
    if (wants(node, 'mem')) {
      parts.push(metricHTML('MEM', node.mem_used));
    }
    if (wants(node, 'swap')) {
      parts.push(metricHTML('SWAP', node.swap_used));
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
    const role = node.reachable ? '' : ' role="status"';
    return (
      '<li class="' + cls + '" data-name="' + escapeHTML(node.name) + '">' +
      '<a class="badge" href="' + url + '" title="Open ' + escapeHTML(node.name) + ' badge PNG">' +
      '<img src="' + url + '" alt="' + escapeHTML(node.name) + ' status badge" width="128">' +
      '</a>' +
      '<p class="' + metricsCls + '"' + role + '>' + metricsHTML(node) + '</p>' +
      '</li>'
    );
  }

  function emptyHTML() {
    return (
      '<p class="empty" role="status">' +
      'No nodes configured. Add entries under <code>nodes</code> in the config YAML, then reload.' +
      '</p>'
    );
  }

  function ensureList() {
    if (!fleetEl) {
      return null;
    }
    let list = fleetEl.querySelector('.node-list');
    if (list) {
      return list;
    }
    const empty = fleetEl.querySelector('.empty');
    if (empty) {
      empty.remove();
    }
    list = document.createElement('ul');
    list.className = 'node-list';
    fleetEl.appendChild(list);
    return list;
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
      anchor.title = 'Open ' + node.name + ' badge PNG';
    }
    if (img) {
      setBadgeSrc(img, url);
      img.alt = node.name + ' status badge';
    }
    if (metrics) {
      metrics.className = node.reachable ? 'metrics' : 'metrics error';
      if (node.reachable) {
        metrics.removeAttribute('role');
      } else {
        metrics.setAttribute('role', 'status');
      }
      metrics.innerHTML = metricsHTML(node);
    }
  }

  function sameNodeOrder(nodes, rows) {
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
        setLive('error', 'refresh failed');
        return;
      }
      const data = await res.json();
      metaEl.textContent = 'Updated ' + fmtTime(data.collected_at);
      syncLiveFromFeed();
      const stackURL = stackBadgeURL(data.collected_at);
      if (stackAnchorEl) {
        stackAnchorEl.href = stackURL;
      }
      if (stackImgEl) {
        setBadgeSrc(stackImgEl, stackURL);
      }
      const nodes = data.nodes || [];
      if (nodes.length === 0) {
        if (fleetEl) {
          fleetEl.innerHTML = emptyHTML();
        }
        return;
      }
      const list = ensureList();
      if (!list) {
        return;
      }
      const rows = list.querySelectorAll('.node-row');
      if (!sameNodeOrder(nodes, rows)) {
        list.innerHTML = nodes.map(renderRow).join('');
        return;
      }
      for (let i = 0; i < nodes.length; i++) {
        updateRow(rows[i], nodes[i]);
      }
    } catch (_) {
      setLive('error', 'refresh failed');
    }
  }

  evtSource.addEventListener('update', refresh);
  evtSource.onopen = function () {
    setLive('live', 'live');
  };
  evtSource.onerror = function () {
    setLive('reconnecting', 'reconnecting');
  };
})();
