// WebSocket connection
let ws;
let isScanning = false;

const sortState = {
    key: 'none', // 'none' | 'ping' | 'speed'
    dir: 'none'  // 'none' | 'asc' | 'desc'
};

// Default IP input should be empty
const defaultIPs = ``;

// DOM Elements
const ipInput = document.getElementById('ip-ranges');
const portInput = document.getElementById('port');
const urlInput = document.getElementById('speed-url');
const maxLatencyInput = document.getElementById('max-latency');
const resultsBody = document.getElementById('results-body');
const btnPing = document.getElementById('btn-ping');
const btnSpeed = document.getElementById('btn-speed');
const statusText = document.getElementById('status-text');
const thPing = document.getElementById('th-ping');
const thSpeed = document.getElementById('th-speed');

// Initialize
function init() {
    ipInput.value = defaultIPs;
    connectWS();
}

function connectWS() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

    ws.onopen = () => {
        updateStatus('Connected to server', 'success');
    };

    ws.onclose = () => {
        updateStatus('Disconnected. Reconnecting...', 'error');
        setTimeout(connectWS, 1000);
    };

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);

        if (data.status === 'done') {
            isScanning = false;
            enableButtons();
            updateStatus('Scan complete', 'success');
            return;
        }

        // Handle result
        addResultRow(data);
    };
}

function startScan(type) {
    if (isScanning) return;

    const port = parseInt(portInput.value);
    const targets = parseInputTargets();

    resetColumn(type, targets, port);

    isScanning = true;
    disableButtons();
    updateStatus(`Starting ${type} test...`, 'info');

    const downloadUrl = urlInput.value;
    const maxLatency = parseInt(maxLatencyInput.value);

    // Filter IPs? We send raw text, server parses.

    const req = {
        type: type,
        ips: targets,
        port: port,
        download_url: downloadUrl,
        max_latency: maxLatency
    };

    ws.send(JSON.stringify(req));
}

function retestSingle(ip, port, type) {
    if (isScanning) return;

    const downloadUrl = urlInput.value;
    const req = {
        type: type,
        ips: [ip],
        port: port,
        download_url: downloadUrl,
        max_latency: 9999
    };
    ws.send(JSON.stringify(req));
}

function setSort(key, dir) {
    sortState.key = key;
    sortState.dir = dir;
    applySort();
}

function parseInputTargets() {
    const lines = ipInput.value
        .split('\n')
        .map(line => line.trim())
        .filter(line => line !== '');
    return Array.from(new Set(lines));
}

function resetColumn(type, targets, port) {
    if (!Array.isArray(targets) || targets.length === 0) return;

    for (const target of targets) {
        const row = document.getElementById(safeRowId(target, port));
        if (!row) continue;

        if (type === 'ping') {
            const pingCell = row.querySelector('.ping-cell');
            if (pingCell) {
                pingCell.textContent = '-';
            }
            row.dataset.ping = '';
        } else if (type === 'speed') {
            const speedCell = row.querySelector('.speed-cell');
            if (speedCell) {
                speedCell.textContent = '-';
            }
            row.dataset.speed = '';
        }

        const statusCell = row.querySelector('.status-cell');
        if (statusCell) {
            statusCell.innerHTML = '<span class="status-badge">PENDING</span>';
        }
    }

    if (sortState.key === type && sortState.dir !== 'none') {
        applySort();
    }
}

function cycleSort(key) {
    if (sortState.key !== key) {
        setSort(key, 'asc');
        return;
    }

    if (sortState.dir === 'asc') {
        setSort(key, 'desc');
        return;
    }

    if (sortState.dir === 'desc') {
        setSort('none', 'none');
        return;
    }

    setSort(key, 'asc');
}

function applySort() {
    const rows = Array.from(resultsBody.querySelectorAll('tr'));
    if (rows.length <= 1) return;

    const byOrderAsc = (a, b) => {
        const ao = Number(a.dataset.order ?? Number.MAX_SAFE_INTEGER);
        const bo = Number(b.dataset.order ?? Number.MAX_SAFE_INTEGER);
        return ao - bo;
    };

    let comparator = byOrderAsc;

    if (sortState.key === 'ping' && sortState.dir !== 'none') {
        comparator = (a, b) => {
            const ap = a.dataset.ping === '' || a.dataset.ping == null ? Number.POSITIVE_INFINITY : Number(a.dataset.ping);
            const bp = b.dataset.ping === '' || b.dataset.ping == null ? Number.POSITIVE_INFINITY : Number(b.dataset.ping);
            const diff = sortState.dir === 'asc' ? (ap - bp) : (bp - ap);
            return diff !== 0 ? diff : byOrderAsc(a, b);
        };
    } else if (sortState.key === 'speed' && sortState.dir !== 'none') {
        comparator = (a, b) => {
            const as = a.dataset.speed === '' || a.dataset.speed == null ? Number.NEGATIVE_INFINITY : Number(a.dataset.speed);
            const bs = b.dataset.speed === '' || b.dataset.speed == null ? Number.NEGATIVE_INFINITY : Number(b.dataset.speed);
            const diff = sortState.dir === 'asc' ? (as - bs) : (bs - as);
            return diff !== 0 ? diff : byOrderAsc(a, b);
        };
    }

    rows.sort(comparator);
    for (const row of rows) {
        resultsBody.appendChild(row);
    }
}

function safeRowId(ip, port) {
    const safe = encodeURIComponent(String(ip)).replace(/%/g, '_');
    return `row-${safe}-${port}`;
}

function addResultRow(res) {
    // Check if row exists
    const id = safeRowId(res.ip, res.port);
    let tr = document.getElementById(id);

    if (!tr) {
        // Create new row
        tr = document.createElement('tr');
        tr.id = id;

        if (res.order !== undefined && res.order !== null) {
            tr.dataset.order = String(res.order);
        }
        tr.dataset.ping = '';
        tr.dataset.speed = '';

        // Initial structure
        tr.innerHTML = `
            <td>${res.ip}</td>
            <td>${res.port}</td>
            <td class="ping-cell clickable" title="Click to retest ping">-</td>
            <td class="speed-cell clickable" title="Click to retest speed">-</td>
            <td class="status-cell"><span class="status-badge">PENDING</span></td>
        `;

        // Add click handlers for single retest
        tr.querySelector('.ping-cell').addEventListener('click', () => retestSingle(res.ip, res.port, 'ping'));
        tr.querySelector('.speed-cell').addEventListener('click', () => retestSingle(res.ip, res.port, 'speed'));

        // Insert in original input order if we have it
        if (tr.dataset.order !== '' && tr.dataset.order != null) {
            const order = Number(tr.dataset.order);
            const existing = Array.from(resultsBody.querySelectorAll('tr'));
            const insertBefore = existing.find(r => Number(r.dataset.order ?? Number.MAX_SAFE_INTEGER) > order);
            if (insertBefore) {
                resultsBody.insertBefore(tr, insertBefore);
            } else {
                resultsBody.appendChild(tr);
            }
        } else {
            resultsBody.appendChild(tr);
        }
    }

    // Update data safely
    const pingCell = tr.querySelector('.ping-cell');
    const speedCell = tr.querySelector('.speed-cell');
    const statusCell = tr.querySelector('.status-cell');

    if (res.ping_time > 0) {
        const latencyClass = res.ping_time < 100 ? 'latency-good' : (res.ping_time > 300 ? 'latency-bad' : '');
        pingCell.innerHTML = `<span class="${latencyClass}">${res.ping_time} ms</span>`;
        tr.dataset.ping = String(res.ping_time);
    }

    if (res.download > 0) {
        speedCell.textContent = res.download.toFixed(2) + ' MB/s';
        tr.dataset.speed = String(res.download);
    }

    // Update status
    statusCell.innerHTML = `<span class="status-badge ${res.status === 'ok' ? 'status-ok' : 'status-error'}">${res.status}</span>`;

    // Keep current sort applied when values update
    if (sortState.key !== 'none' && sortState.dir !== 'none') {
        applySort();
    }
}

function updateStatus(msg, type) {
    statusText.textContent = msg;
    const normalized = (type || 'info').toLowerCase();
    const allowed = new Set(['info', 'success', 'error']);
    const safeType = allowed.has(normalized) ? normalized : 'info';
    statusText.className = `status-text status-${safeType}`;
}

function disableButtons() {
    btnPing.disabled = true;
    btnSpeed.disabled = true;
    btnPing.style.opacity = 0.5;
    btnSpeed.style.opacity = 0.5;
}

function enableButtons() {
    btnPing.disabled = false;
    btnSpeed.disabled = false;
    btnPing.style.opacity = 1;
    btnSpeed.style.opacity = 1;
}

// Event Listeners
btnPing.addEventListener('click', () => startScan('ping'));
btnSpeed.addEventListener('click', () => startScan('speed'));

thPing?.addEventListener('click', () => cycleSort('ping'));
thSpeed?.addEventListener('click', () => cycleSort('speed'));

init();
