const app = () => window.go?.main?.App;
let catalog = null;
const selectedPlugins = new Set();
const EXT_PREFS_KEY = 'sysinitpro_v10_prefs_v1';

function $(id) { return document.getElementById(id); }

function loadLocalPrefs() {
  try {
    return JSON.parse(localStorage.getItem(EXT_PREFS_KEY) || '{}');
  } catch (_) {
    return {};
  }
}

function saveLocalPrefs(patch) {
  try {
    const cur = loadLocalPrefs();
    localStorage.setItem(EXT_PREFS_KEY, JSON.stringify({ ...cur, ...patch }));
  } catch (_) {}
}

function appendLog(line) {
  const el = $('v10-log-console');
  if (!el) return;
  el.textContent += (el.textContent ? '\n' : '') + line;
  el.scrollTop = el.scrollHeight;
}

function setTaskUI(state, text) {
  const badge = $('task-badge');
  badge.className = 'status-pill ' + state;
  badge.textContent = text;
  const running = state === 'running';
  $('btn-owrt-build').disabled = running;
  $('btn-owrt-cancel').disabled = !running;
}

// --- 高危操作二次确认 ---
let pendingDangerFn = null;
function confirmDangerAction(title, desc, fn) {
  $('confirm-title').textContent = '⚠️ ' + title;
  $('confirm-desc').textContent = desc;
  pendingDangerFn = fn;
  $('confirm-modal').style.display = 'flex';
}
$('btn-confirm-cancel')?.addEventListener('click', () => {
  pendingDangerFn = null;
  $('confirm-modal').style.display = 'none';
});
$('btn-confirm-ok')?.addEventListener('click', async () => {
  const fn = pendingDangerFn;
  pendingDangerFn = null;
  $('confirm-modal').style.display = 'none';
  if (fn) await fn();
});

// --- 标签页切换 ---
document.querySelectorAll('.tab-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
    btn.classList.add('active');
    $(btn.dataset.tab).classList.add('active');
  });
});

// --- OpenWrt 逻辑 ---
function getActiveFeeds() {
  const feeds = new Set(['official']);
  document.querySelectorAll('#owrt-feeds-box input[type="checkbox"]').forEach(cb => {
    if (cb.checked) feeds.add(cb.value);
  });
  return feeds;
}

function getVisiblePlugins() {
  if (!catalog) return [];
  const activeFeeds = getActiveFeeds();
  const kw = ($('owrt-search')?.value || '').trim().toLowerCase();
  const onlySel = $('owrt-only-selected')?.checked;
  const seen = new Set();
  return catalog.plugins.filter(p => {
    if (!activeFeeds.has(p.feed)) return false;
    if (seen.has(p.id)) return false;
    seen.add(p.id);
    if (onlySel && !selectedPlugins.has(p.id)) return false;
    if (kw && !p.id.toLowerCase().includes(kw) && !p.name.toLowerCase().includes(kw) && !p.desc.toLowerCase().includes(kw)) {
      return false;
    }
    return true;
  });
}

async function refreshConflictBanner() {
  const box = $('owrt-conflict-banner');
  if (!box || !app()?.V10CheckOpenWrtConflicts) return;
  try {
    const list = await app().V10CheckOpenWrtConflicts(collectFirmwareConfig());
    if (!list || list.length === 0) {
      box.style.display = 'none';
      box.innerHTML = '';
      return;
    }
    const hasErr = list.some(c => c.level === 'error');
    box.style.display = 'block';
    box.style.background = hasErr ? 'rgba(239,68,68,0.14)' : 'rgba(245,158,11,0.14)';
    box.style.border = hasErr ? '1px solid rgba(239,68,68,0.5)' : '1px solid rgba(245,158,11,0.5)';
    box.style.color = hasErr ? '#fca5a5' : '#fde68a';
    box.innerHTML = list.map(c =>
      `<div><strong>${c.level === 'error' ? '🔴' : '🟡'} ${c.title}：</strong>${c.message}</div>`
    ).join('');
  } catch (_) {}
}

function renderPlugins() {
  if (!catalog) return;
  const grid = $('owrt-plugin-grid');
  const activeFeeds = getActiveFeeds();
  const feedTotal = catalog.plugins.filter(p => activeFeeds.has(p.feed));
  const feedIds = new Set(feedTotal.map(p => p.id));
  for (const id of Array.from(selectedPlugins)) {
    if (!feedIds.has(id)) selectedPlugins.delete(id);
  }
  const visible = getVisiblePlugins();
  $('owrt-count-sel').textContent = selectedPlugins.size;
  $('owrt-count-all').textContent = feedTotal.length;

  grid.innerHTML = '';
  const frag = document.createDocumentFragment();
  visible.forEach(p => {
    const on = selectedPlugins.has(p.id);
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'plugin-card' + (on ? ' active' : '');
    btn.innerHTML = `
      <div class="p-top"><span>${on ? '✅ ' : '⬜ '}${p.name}</span><span class="p-tag">${p.feed}</span></div>
      <div style="font-size:11px;color:#64748b;font-family:Consolas,monospace;margin:2px 0 4px;">${p.id}</div>
      <div class="p-desc">${p.desc}</div>
    `;
    btn.addEventListener('click', () => {
      if (selectedPlugins.has(p.id)) selectedPlugins.delete(p.id);
      else selectedPlugins.add(p.id);
      renderPlugins();
    });
    frag.appendChild(btn);
  });
  grid.appendChild(frag);
  refreshConflictBanner();
}

function collectFirmwareConfig() {
  return {
    target: $('owrt-target').value,
    branch: $('owrt-branch').value,
    lanIP: $('owrt-ip').value.trim() || '192.168.100.1',
    rootPassword: $('owrt-pass').value.trim(),
    theme: $('owrt-theme').value,
    gitMirror: $('owrt-mirror').value.trim(),
    feeds: Array.from(getActiveFeeds()),
    plugins: Array.from(selectedPlugins)
  };
}

async function initV10() {
  if (!app()) return;
  window.runtime?.EventsOn?.('v10:log', (line) => appendLog(line));

  catalog = await app().V10GetOpenWrtCatalog();
  const saved = await app().V10LoadConfig();
  const tpls = await app().V10GetStructTemplates();
  const prefs = loadLocalPrefs();

  $('pve-cluster-json').value = prefs.pveJson || JSON.stringify(tpls.pve, null, 2);
  $('vps-sec-json').value = prefs.vpsJson || JSON.stringify(tpls.vps, null, 2);
  if (prefs.ghRepo) $('gh-repo').value = prefs.ghRepo;

  if (app().V10LoadSecret) {
    const savedTok = await app().V10LoadSecret('ghToken');
    if (savedTok) $('gh-token').value = savedTok;
  }

  $('owrt-target').innerHTML = catalog.targets.map(t => `<option value="${t.id}">${t.name}</option>`).join('');
  $('owrt-branch').innerHTML = catalog.branches.map(b => `<option value="${b.id}">${b.name}</option>`).join('');
  $('owrt-theme').innerHTML = catalog.themes.map(t => `<option value="${t.id}">${t.name}</option>`).join('');

  const feedsBox = $('owrt-feeds-box');
  feedsBox.innerHTML = '<span style="font-weight:600;">第三方插件源：</span>' + catalog.feeds.map(f => {
    const checked = saved?.openwrt?.feeds?.length
      ? saved.openwrt.feeds.includes(f.id)
      : f.id === 'kenzok8';
    return `<label><input type="checkbox" value="${f.id}" ${checked ? 'checked' : ''} /> ${f.name}</label>`;
  }).join('');
  feedsBox.querySelectorAll('input').forEach(cb => cb.addEventListener('change', renderPlugins));

  if (saved?.openwrt?.target) $('owrt-target').value = saved.openwrt.target;
  if (saved?.openwrt?.branch) $('owrt-branch').value = saved.openwrt.branch;
  if (saved?.openwrt?.theme) $('owrt-theme').value = saved.openwrt.theme;
  $('owrt-ip').value = saved?.openwrt?.lanIP || '192.168.100.1';
  $('owrt-pass').value = saved?.openwrt?.rootPassword || '';
  $('owrt-mirror').value = saved?.openwrt?.gitMirror || '';

  if (saved?.openwrt?.plugins?.length) {
    saved.openwrt.plugins.forEach(id => selectedPlugins.add(id));
  } else {
    catalog.plugins.filter(p => p.defaultOn && p.feed !== 'istore').forEach(p => selectedPlugins.add(p.id));
  }

  if (saved?.pveHost) $('pve-host').value = saved.pveHost;
  if (saved?.pveUser) $('pve-user').value = saved.pveUser;

  renderPlugins();
}

// --- Tab 1: 系统初始化事件 ---
function renderHardwareCards(hw) {
  const box = $('hw-cards');
  if (!box || !hw || typeof hw !== 'object') return;
  const entries = Object.entries(hw).filter(([_, v]) => v !== '' && v !== null && v !== undefined);
  if (entries.length === 0) { box.innerHTML = ''; return; }
  box.innerHTML = entries.slice(0, 6).map(([k, v]) => {
    const valStr = typeof v === 'object' ? JSON.stringify(v) : String(v);
    return `<div style="background:#0f172a;border:1px solid var(--border);border-radius:8px;padding:10px;">
      <div style="font-size:11px;color:var(--muted);">${k}</div>
      <div style="font-size:13px;font-weight:600;margin-top:4px;word-break:break-all;">${valStr}</div>
    </div>`;
  }).join('');
}

$('btn-scan-hw').addEventListener('click', async () => {
  $('sysinit-console').textContent = '正在扫描本机硬件信息...';
  try {
    const hw = await app().V10ScanHardware();
    renderHardwareCards(hw);
    $('sysinit-console').textContent = JSON.stringify(hw, null, 2);
  } catch (e) { $('sysinit-console').textContent = '[FAIL] ' + e; }
});

$('btn-preflight').addEventListener('click', async () => {
  $('sysinit-console').textContent = '正在执行环境预检...';
  const res = await app().V10RunPreflight();
  $('sysinit-console').textContent = JSON.stringify(res, null, 2);
});

$('btn-scan-patches').addEventListener('click', async () => {
  $('sysinit-console').textContent = '正在扫描系统可用补丁...';
  try {
    const list = await app().V10ScanPatches();
    $('sysinit-console').textContent = JSON.stringify(list, null, 2);
  } catch (e) { $('sysinit-console').textContent = '[FAIL] ' + e; }
});

$('btn-install-patches').addEventListener('click', async () => {
  $('sysinit-console').textContent = '正在安装安全补丁（请稍候）...';
  $('sysinit-console').textContent = await app().V10InstallPatches(true);
});

$('btn-install-runtimes').addEventListener('click', async () => {
  $('sysinit-console').textContent = '正在安装系统运行库...';
  $('sysinit-console').textContent = await app().V10InstallRuntimes();
});

$('btn-bench-mirrors').addEventListener('click', async () => {
  $('sysinit-console').textContent = '正在测速最优镜像节点...';
  const best = await app().V10BenchmarkMirrors();
  $('sysinit-console').textContent = '最优镜像节点:\n' + JSON.stringify(best, null, 2);
});

$('btn-search-soft').addEventListener('click', async () => {
  const kw = $('soft-keyword').value.trim();
  if (!kw) return;
  $('soft-console').textContent = '正在搜索 ' + kw + ' ...';
  try {
    $('soft-console').textContent = await app().V10SearchSoftware(kw);
  } catch (e) { $('soft-console').textContent = '[FAIL] ' + e; }
});

// --- Tab 2: OpenWrt 事件 ---
$('owrt-search').addEventListener('input', renderPlugins);
$('owrt-only-selected').addEventListener('change', renderPlugins);
$('owrt-branch').addEventListener('change', refreshConflictBanner);

async function runFeedSync(allowPrompt) {
  const btn = $('btn-sync-feeds');
  btn.disabled = true; btn.textContent = '⏳ 同步中...';
  appendLog('>>> 正在同步官方/KenzoK8/iStore 全量插件索引...');
  try {
    const token = ($('gh-token')?.value || '').trim();
    if (app().V10SaveSecret) await app().V10SaveSecret('ghToken', token);
    const res = await app().V10SyncOpenWrtFeeds(token);
    if (res.ok && res.plugins) {
      catalog.plugins = res.plugins;
      renderPlugins();
      appendLog('[OK] ' + res.message);
    } else {
      appendLog('[FAIL] 同步失败: ' + (res.message || '未知错误'));
    }
    if (res.rateLimited && allowPrompt) {
      $('modal-gh-token').value = token;
      $('token-modal').style.display = 'flex';
    }
  } catch (e) {
    appendLog('[FAIL] 前端调用异常: ' + String(e));
  } finally {
    btn.disabled = false; btn.textContent = '🔄 同步全量插件库';
  }
}

$('btn-sync-feeds').addEventListener('click', () => runFeedSync(true));
$('btn-modal-cancel').addEventListener('click', () => { $('token-modal').style.display = 'none'; });
$('btn-open-gh-token')?.addEventListener('click', () => {
  const url = 'https://github.com/settings/tokens/new?description=SysInitPro-v10';
  if (window.runtime?.BrowserOpenURL) window.runtime.BrowserOpenURL(url);
  else window.open(url, '_blank');
});
$('btn-modal-retry').addEventListener('click', async () => {
  const t = $('modal-gh-token').value.trim();
  if (!t) { $('modal-gh-token').focus(); return; }
  $('gh-token').value = t;
  if (app().V10SaveSecret) await app().V10SaveSecret('ghToken', t);
  $('token-modal').style.display = 'none';
  runFeedSync(false);
});

$('btn-gh-push').addEventListener('click', async () => {
  const btn = $('btn-gh-push'); btn.disabled = true;
  const repo = $('gh-repo').value.trim();
  const token = $('gh-token').value.trim();
  saveLocalPrefs({ ghRepo: repo });
  if (app().V10SaveSecret) await app().V10SaveSecret('ghToken', token);
  appendLog('>>> 正在推送编译配置至 GitHub Actions...');
  try {
    appendLog(await app().V10PushOpenWrtGitHub(repo, token, collectFirmwareConfig()));
  } finally { btn.disabled = false; }
});

$('btn-owrt-all').addEventListener('click', () => { getVisiblePlugins().forEach(p => selectedPlugins.add(p.id)); renderPlugins(); });
$('btn-owrt-clear').addEventListener('click', () => { selectedPlugins.clear(); renderPlugins(); });

$('btn-owrt-save').addEventListener('click', async () => {
  saveLocalPrefs({
    ghRepo: $('gh-repo').value.trim(),
    pveJson: $('pve-cluster-json').value,
    vpsJson: $('vps-sec-json').value
  });
  if (app().V10SaveSecret) await app().V10SaveSecret('ghToken', $('gh-token').value.trim());
  await app().V10SaveConfig({
    openwrt: collectFirmwareConfig(),
    pveHost: $('pve-host').value.trim(),
    pveUser: $('pve-user').value.trim() || 'root'
  });
  appendLog('[OK] 当前配置已保存到本地（Token 已加密存储）');
});

$('btn-owrt-preview').addEventListener('click', async () => {
  const bundle = await app().V10PreviewOpenWrtBundle(collectFirmwareConfig());
  $('preview-card').style.display = 'block';
  $('preview-console').textContent = `=== diy-part1.sh ===\n${bundle.diyPart1}\n=== diy-part2.sh ===\n${bundle.diyPart2}\n=== diff.config ===\n${bundle.dotConfig}`;
});

$('btn-close-preview').addEventListener('click', () => { $('preview-card').style.display = 'none'; });
$('btn-clear-log').addEventListener('click', () => { $('v10-log-console').textContent = ''; });

$('btn-owrt-build').addEventListener('click', async () => {
  const cfg = collectFirmwareConfig();
  await app().V10SaveConfig({ openwrt: cfg, pveHost: $('pve-host').value.trim(), pveUser: $('pve-user').value.trim() || 'root' });
  $('v10-log-console').textContent = '>>> 正在初始化 v10 编译引擎...\n';
  setTaskUI('running', '编译中...');
  try {
    const res = await app().V10StartOpenWrtBuild(cfg);
    appendLog(res);
    setTaskUI(res.startsWith('[OK]') ? 'success' : 'failed', res.startsWith('[OK]') ? '完成' : '失败');
  } catch (e) { appendLog('[FAIL] ' + String(e)); setTaskUI('failed', '异常'); }
});

$('btn-owrt-cancel').addEventListener('click', async () => {
  if (await app().V10CancelTask()) { appendLog('⚠️ 已发送中止信号...'); setTaskUI('canceled', '已中止'); }
});

// --- Tab 3: PVE 体检与编排事件 ---
$('btn-pve-probe').addEventListener('click', async () => {
  const btn = $('btn-pve-probe'); btn.disabled = true;
  $('pve-result').textContent = '正在连接并探测主机状态（最多 10 秒）...';
  try {
    const res = await app().V10ProbePVE($('pve-host').value, $('pve-user').value, $('pve-pass').value);
    $('pve-result').textContent = res.ok ? `[OK] 连接成功（耗时 ${res.latency}）\n${res.summary}` : `[FAIL] ${res.error}`;
  } finally { btn.disabled = false; }
});

$('btn-pve-fill-from-ssh')?.addEventListener('click', () => {
  try {
    const cur = JSON.parse($('pve-cluster-json').value || '{}');
    const rawHost = $('pve-host').value.trim();
    const parts = rawHost.split(':');
    cur.Host = parts[0] || cur.Host || '';
    if (parts[1] && Number(parts[1]) > 0) cur.Port = Number(parts[1]);
    cur.User = $('pve-user').value.trim() || cur.User || 'root';
    if ($('pve-pass').value) cur.Password = $('pve-pass').value;
    $('pve-cluster-json').value = JSON.stringify(cur, null, 2);
    saveLocalPrefs({ pveJson: $('pve-cluster-json').value });
    $('pve-orch-console').textContent = '[OK] 已将上方 SSH 连接信息同步至编排配置';
  } catch (e) { $('pve-orch-console').textContent = '[FAIL] ' + e; }
});

async function runPVEAction(fn, label) {
  $('pve-orch-console').textContent = `正在执行 ${label}...`;
  try {
    const cfg = JSON.parse($('pve-cluster-json').value);
    saveLocalPrefs({ pveJson: $('pve-cluster-json').value });
    $('pve-orch-console').textContent = await fn(cfg);
  } catch (e) { $('pve-orch-console').textContent = '[FAIL] ' + e; }
}

$('btn-pve-upgrade-host').addEventListener('click', () => confirmDangerAction(
  '确认升级 PVE 宿主机',
  '即将对目标 PVE 宿主机执行系统升级（可能触发内核更新），请确认已备份关键虚拟机。',
  () => runPVEAction(c => app().V10UpgradePVEHost(c), '升级 PVE 宿主机')
));
$('btn-pve-boot-order').addEventListener('click', () => runPVEAction(c => app().V10ConfigurePVEBootOrder(c), '配置 VM 启动顺序'));
$('btn-pve-upgrade-vms').addEventListener('click', () => confirmDangerAction(
  '确认安全升级虚拟机',
  '即将按配置对集群内虚拟机执行批量更新，请确认业务窗口允许短暂重启。',
  () => runPVEAction(c => app().V10SafeUpgradePVEVMs(c), '安全升级虚拟机')
));

// --- Tab 4: Sing-box 与 VPS 安全事件 ---
$('btn-sb-parse').addEventListener('click', async () => {
  const uri = $('sb-uri').value.trim();
  if (!uri) return;
  $('sb-console').textContent = '正在解析节点并生成 Sing-box JSON...';
  try {
    const parsed = await app().V10ParseSingboxURI(uri);
    const jsonStr = await app().V10GenerateSingboxJSON(parsed);
    $('sb-console').textContent = jsonStr;
  } catch (e) { $('sb-console').textContent = '[FAIL] ' + e; }
});

$('btn-sb-copy')?.addEventListener('click', async () => {
  const txt = $('sb-console').textContent;
  if (!txt || txt.startsWith('等待')) return;
  try {
    await navigator.clipboard.writeText(txt);
    $('sb-console').textContent = txt + '\n\n[OK] 已复制到剪贴板';
  } catch (_) {}
});

$('btn-vps-gen').addEventListener('click', async () => {
  try {
    const cfg = JSON.parse($('vps-sec-json').value);
    saveLocalPrefs({ vpsJson: $('vps-sec-json').value });
    $('vps-console').textContent = await app().V10GenerateVPSSecurity(cfg);
  } catch (e) { $('vps-console').textContent = '[FAIL] ' + e; }
});

$('btn-vps-run').addEventListener('click', () => confirmDangerAction(
  '确认执行 VPS SSH 安全加固',
  '即将修改远程 VPS 的 SSH 端口/认证策略与防火墙规则，请确认已保留备用登录通道。',
  async () => {
    $('vps-console').textContent = '正在通过 SSH 执行 VPS 安全加固...';
    try {
      const cfg = JSON.parse($('vps-sec-json').value);
      saveLocalPrefs({ vpsJson: $('vps-sec-json').value });
      $('vps-console').textContent = await app().V10RunVPSSSHProvision(cfg);
    } catch (e) { $('vps-console').textContent = '[FAIL] ' + e; }
  }
));

// --- Tab 5: 完整性校验 ---
$('btn-check-integrity').addEventListener('click', async () => {
  $('integrity-console').textContent = '正在核验...';
  $('integrity-console').textContent = await app().CheckCodeIntegrity();
});

$('btn-refresh-manifest').addEventListener('click', async () => {
  $('integrity-console').textContent = '正在刷新清单...';
  $('integrity-console').textContent = await app().RefreshCodeManifest();
});

window.addEventListener('DOMContentLoaded', initV10);
$('btn-i18n-fill')?.addEventListener('click', async () => {
  const btn = $('btn-i18n-fill');
  btn.disabled = true; btn.textContent = '⏳ 补全中...';
  appendLog('>>> 正在自动补全缺失插件的中文名与说明...');
  try {
    const res = await app().V10TranslateMissingPlugins();
    if (res.ok && res.plugins) {
      catalog.plugins = res.plugins;
      renderPlugins();
      appendLog('[OK] ' + res.message);
    } else {
      appendLog('[FAIL] 补全失败: ' + (res.message || '未知错误'));
    }
  } catch (e) {
    appendLog('[FAIL] ' + String(e));
  } finally {
    btn.disabled = false; btn.textContent = '🌐 补全缺失中文';
  }
});