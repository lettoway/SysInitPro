let currentHW = null, currentMirror = "official", currentRole = "desktop", catalog = [], localSSHHosts = [], selectedIdentityFile = "";
const goApp = () => window.go.main.App;

function escapeHtml(s) {
  return String(s || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function waitForWails() {
  return new Promise(r => { const c = () => (window.go && window.go.main && window.go.main.App) ? r() : setTimeout(c, 50); c(); });
}

async function restoreUIState() {
  try {
    const st = await goApp().LoadUIState();
    if (!st) return;
    document.querySelectorAll('.persist-field').forEach(el => { if (st[el.id] !== undefined) el.value = st[el.id]; });
  } catch (e) {}
}

function bindUIStatePersistence() {
  document.querySelectorAll('.persist-field').forEach(el => {
    el.addEventListener('change', () => {
      const st = {};
      document.querySelectorAll('.persist-field').forEach(f => { st[f.id] = f.value; });
      goApp().SaveUIState(st).catch(() => {});
    });
  });
}

function renderRoleOptimizations(role) {
  const c = document.getElementById('dynamic-opt-card');
  c.innerHTML = role === 'server'
    ? `<h4 class="card-title" style="color:#38bdf8;">🖧 服务器内核调优</h4>
       <label class="inline-check"><input type="checkbox" class="opt-item" value="srv_bbr" checked /> 开启 TCP BBR 与 IP 转发</label> &nbsp;
       <label class="inline-check"><input type="checkbox" class="opt-item" value="srv_limits" checked /> 提升 ulimit 65535</label> &nbsp;
       <label class="inline-check"><input type="checkbox" class="opt-item" value="srv_ssh_harden" checked /> 安装 Fail2Ban</label>`
    : `<h4 class="card-title" style="color:#34d399;">🖥️ 桌面性能调优</h4>
       <label class="inline-check"><input type="checkbox" class="opt-item" value="desk_perf" checked /> 卓越/高性能电源模式</label> &nbsp;
       <label class="inline-check"><input type="checkbox" class="opt-item" value="desk_telemetry" checked /> 关闭冗余遥测与显示后缀</label> &nbsp;
       <label class="inline-check"><input type="checkbox" class="opt-item" value="desk_multimedia" checked /> 补齐中文字体与解码库</label>`;
}

async function loadLocalSSHHosts() {
  try {
    localSSHHosts = (await goApp().ListSSHHosts()) || [];
    document.getElementById('ssh-host-select').innerHTML = `<option value="">-- 选择 ~/.ssh/config 主机 (${localSSHHosts.length}) --</option>` +
      localSSHHosts.map((h, i) => `<option value="${i}">${escapeHtml(h.alias)} (${escapeHtml(h.user)}@${escapeHtml(h.hostname)}:${escapeHtml(h.port)})</option>`).join('');
  } catch (e) {}
}

window.addEventListener('DOMContentLoaded', async () => {
  document.querySelectorAll('.nav-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.nav-btn, .step-panel').forEach(el => el.classList.remove('active'));
      btn.classList.add('active');
      document.getElementById(`step-${btn.dataset.step}`).classList.add('active');
    });
  });
  renderRoleOptimizations(currentRole);
  await waitForWails();
  await restoreUIState();
  bindUIStatePersistence();

  const mirrors = await goApp().GetAvailableMirrors();
  const sel = document.getElementById('mirror-select');
  sel.innerHTML = mirrors.map(m => `<option value="${escapeHtml(m.base_url)}">${escapeHtml(m.name)}</option>`).join('');
  sel.addEventListener('change', e => currentMirror = e.target.value);

  catalog = await goApp().GetSoftwareCatalog();
  await loadLocalSSHHosts();

  try {
    currentHW = await goApp().ScanHardware();
    document.getElementById('os-badge').innerText = `${(currentHW.os_platform || 'OS').toUpperCase()} · ${(currentHW.os_role || 'desktop').toUpperCase()}`;
    if (currentHW.os_role) { currentRole = currentHW.os_role; document.getElementById('os-role-select').value = currentRole; renderRoleOptimizations(currentRole); }
    document.getElementById('hw-card').innerHTML = `<div class="grid-3">
      <p><b>系统：</b>${escapeHtml(currentHW.os_version)} [${escapeHtml(currentHW.activation_status)}]</p>
      <p><b>CPU：</b>${escapeHtml(currentHW.cpu)}</p><p><b>GPU：</b>${escapeHtml(currentHW.gpu)}</p>
      <p><b>主板：</b>${escapeHtml(currentHW.motherboard)}</p><p><b>网卡：</b>${escapeHtml(currentHW.network_card)}</p>
    </div>`;
  } catch (e) { document.getElementById('hw-card').innerText = "硬件扫描失败: " + e; }
  renderCatalog();

  if (window.runtime && window.runtime.EventsOn) {
    window.runtime.EventsOn("task:progress", d => {
      if (d.percent >= 0) document.getElementById('progress-bar').style.width = `${d.percent}%`;
      if (d.percent >= 100) { const b = document.getElementById('btn-start'); b.disabled = false; b.innerText = "🚀 立即在本机执行初始化流水线"; }
      const el = document.getElementById('log-console');
      el.textContent += `\n${d.percent >= 0 ? `[${d.percent}%] ` : ""}${d.message}`;
      el.scrollTop = el.scrollHeight;
    });
    window.runtime.EventsOn("openwrt:log", msg => {
      const box = document.getElementById('ow-script-preview'); box.style.display = 'block'; box.textContent += `\n${msg}`; box.scrollTop = box.scrollHeight;
    });
  }
});

// ================= 系统补丁扫描与分级安装 =================
document.getElementById('btn-scan-patches').addEventListener('click', async () => {
  const box = document.getElementById('patch-console'); box.style.display = 'block';
  box.textContent = "🔍 正在扫描当前系统缺失的补丁（Windows Update / apt / softwareupdate）...";
  try {
    const patches = await goApp().ScanSystemPatches();
    if (!patches || patches.length === 0) { box.textContent = "✅ 系统已是最新状态，未发现待安装补丁。"; return; }
    box.textContent = `发现 ${patches.length} 个待安装补丁：\n` + patches.map(p => `[${p.category}] ${p.id} - ${p.title}`).join("\n");
  } catch (e) { box.textContent = "❌ 扫描失败: " + e; }
});

document.getElementById('btn-install-patches').addEventListener('click', async () => {
  const box = document.getElementById('patch-console'); box.style.display = 'block';
  const secOnly = document.getElementById('chk-sec-only').checked;
  box.textContent = secOnly ? "⚡ 正在静默安装安全与质量补丁（禁止自动重启）..." : "⚡ 正在安装全部可用系统补丁...";
  try { box.textContent = await goApp().InstallSystemPatchesNow(secOnly); }
  catch (e) { box.textContent = "❌ 安装失败: " + e; }
});

// ================= 常用软件与环境 =================
function renderCatalog() {
  const c = document.getElementById('software-list');
  c.innerHTML = catalog.map((item, idx) => {
    const chk = ((item.target_role || "all") === "all" || item.target_role === currentRole) ? 'checked' : '';
    return `<label class="checkbox-card"><input type="checkbox" class="sw-item" value="${escapeHtml(item.id)}" ${chk} />
      <span><b>${escapeHtml(item.name)}</b><br/><small class="sub-text">${escapeHtml(item.targets.winget || item.targets.apt || item.targets.brew || '')}</small></span>
      <button type="button" class="del-sw-btn" data-idx="${idx}">✕</button></label>`;
  }).join('');
  document.querySelectorAll('.del-sw-btn').forEach(b => b.addEventListener('click', async e => {
    e.preventDefault(); catalog.splice(parseInt(b.dataset.idx), 1); await goApp().SaveSoftwareCatalog(catalog); renderCatalog();
  }));
}

function collectProfile() {
  return {
    profile_name: currentRole === "server" ? "服务器生产方案" : "桌面工作站方案",
    target_os: (currentHW && currentHW.os_platform) ? currentHW.os_platform : "windows",
    target_role: currentRole,
    create_backup: document.getElementById('chk-backup').checked,
    install_drivers: document.getElementById('chk-drivers').checked,
    auto_update_os: document.getElementById('chk-os-update').checked,
    security_only: document.getElementById('chk-sec-only').checked,
    install_runtimes: document.getElementById('chk-runtimes').checked,
    optimize_options: Array.from(document.querySelectorAll('.opt-item:checked')).map(el => el.value),
    selected_mirror: currentMirror,
    software_list: Array.from(document.querySelectorAll('.sw-item:checked')).map(el => catalog.find(i => i.id === el.value)).filter(Boolean),
    dev_extra: { git_user_name: document.getElementById('dev-git-name').value.trim(), git_user_email: document.getElementById('dev-git-email').value.trim(), git_proxy_port: document.getElementById('dev-git-proxy').value.trim(), docker_mirror_url: document.getElementById('dev-docker-mirror').value.trim(), go_private_domains: document.getElementById('dev-go-private').value.trim() },
    net_extra: { custom_hostname: document.getElementById('net-hostname').value.trim(), ssh_custom_port: document.getElementById('net-ssh-port').value.trim(), swap_size_gb: parseInt(document.getElementById('net-swap').value || "0"), disable_hibernat: document.getElementById('win-no-hibernate').checked, tailscale_key: document.getElementById('ts-authkey').value.trim(), tailscale_routes: document.getElementById('ts-routes').value.trim(), tailscale_exit: document.getElementById('ts-exit-node').checked }
  };
}

document.getElementById('os-role-select').addEventListener('change', e => { currentRole = e.target.value; renderRoleOptimizations(currentRole); renderCatalog(); });
document.getElementById('btn-toggle-add').addEventListener('click', () => { const p = document.getElementById('add-panel'); p.style.display = p.style.display === 'none' ? 'block' : 'none'; });
document.getElementById('btn-search-pkg').addEventListener('click', async () => { const b = document.getElementById('search-result'); b.style.display = 'block'; b.textContent = await goApp().SearchPackageByWinget(document.getElementById('search-kw').value); });
document.getElementById('btn-confirm-add').addEventListener('click', async () => {
  const name = document.getElementById('new-sw-name').value.trim(); if (!name) return alert("请填名称");
  catalog.push({ id: name.toLowerCase().replace(/[^a-z0-9]/g, '_'), name, category: document.getElementById('new-sw-cat').value, target_role: currentRole, bin_name: document.getElementById('new-sw-bin').value.trim(), targets: { winget: document.getElementById('new-pkg-winget').value.trim(), brew: document.getElementById('new-pkg-brew').value.trim(), apt: document.getElementById('new-pkg-apt').value.trim(), dnf: document.getElementById('new-pkg-dnf').value.trim(), pacman: document.getElementById('new-pkg-pacman').value.trim() } });
  await goApp().SaveSoftwareCatalog(catalog); renderCatalog(); document.getElementById('add-panel').style.display = 'none';
});
document.getElementById('btn-import-local').addEventListener('click', async () => {
  try { const list = await goApp().ImportLocalEnvironment(); const ids = new Set(catalog.map(i => i.id)); let n = 0; (list || []).forEach(s => { if (!ids.has(s.id)) { catalog.push(s); n++; } }); await goApp().SaveSoftwareCatalog(catalog); renderCatalog(); alert(`新增 ${n} 个本机软件！`); } catch (e) { alert(e); }
});
document.getElementById('btn-activate-key').addEventListener('click', async () => { try { await goApp().ActivateWindowsKey(document.getElementById('win-product-key').value); alert("已提交激活！"); } catch (e) { alert(e); } });
document.getElementById('btn-retest-mirror').addEventListener('click', async () => { const m = await goApp().GetFastestMirror(); currentMirror = m.base_url; document.getElementById('mirror-select').value = currentMirror; alert(`最快源: ${m.name}`); });
document.getElementById('btn-preview-playbook').addEventListener('click', async () => { document.getElementById('log-console').textContent = await goApp().PreviewPlaybookScript(collectProfile()); });
document.getElementById('btn-export-playbook').addEventListener('click', async () => { const p = await goApp().ExportPlaybookScript(collectProfile()); if (p) alert("已保存: " + p); });
document.getElementById('btn-verify-env').addEventListener('click', async () => { const r = await goApp().VerifyInstalledEnv(collectProfile().software_list); document.getElementById('log-console').textContent = (r || []).join("\n"); });
document.getElementById('btn-start').addEventListener('click', async () => {
  const b = document.getElementById('btn-start'); b.disabled = true; b.innerText = "⏳ 执行中...";
  document.getElementById('log-console').textContent = "启动初始化流水线...";
  try { await goApp().RunAllTasks(collectProfile(), currentHW); } catch (e) { alert(e); b.disabled = false; b.innerText = "🚀 立即在本机执行初始化流水线"; }
});
document.getElementById('btn-export').addEventListener('click', async () => { const p = await goApp().ExportConfigDialog(collectProfile()); if (p) alert("已脱敏导出: " + p); });
document.getElementById('btn-import').addEventListener('click', async () => { const p = await goApp().ImportConfigDialog(); if (p) alert("已加载: " + p.profile_name); });
document.getElementById('btn-gist-push').addEventListener('click', async () => { try { const r = await goApp().SyncToGitHubGist(document.getElementById('gist-token').value, document.getElementById('gist-id').value, collectProfile(), catalog); document.getElementById('gist-id').value = r.gist_id; alert("Gist 备份成功: " + r.gist_id); } catch (e) { alert(e); } });
document.getElementById('btn-gist-pull').addEventListener('click', async () => { try { await goApp().RestoreFromGitHubGist(document.getElementById('gist-token').value, document.getElementById('gist-id').value); catalog = await goApp().GetSoftwareCatalog(); renderCatalog(); alert("Gist 恢复成功！"); } catch (e) { alert(e); } });

// ================= VPS / Sing-box / OpenWrt / PVE =================
document.getElementById('btn-refresh-hosts').addEventListener('click', loadLocalSSHHosts);
document.getElementById('ssh-host-select').addEventListener('change', e => {
  const h = localSSHHosts[parseInt(e.target.value)]; if (!h) return;
  document.getElementById('vps-alias').value = h.alias; document.getElementById('vps-ip').value = h.hostname;
  document.getElementById('vps-init-user').value = h.user; document.getElementById('vps-old-port').value = h.port;
  selectedIdentityFile = h.identity_file || "";
});
function getVPSTarget() { return { host: document.getElementById('vps-ip').value.trim(), port: document.getElementById('vps-old-port').value.trim() || "22", user: document.getElementById('vps-init-user').value.trim() || "root", password: document.getElementById('vps-init-pass').value, identity_file: selectedIdentityFile }; }
function collectVPS() { return { server_alias: document.getElementById('vps-alias').value.trim(), server_ip: document.getElementById('vps-ip').value.trim(), init_user: document.getElementById('vps-init-user').value.trim(), init_password: document.getElementById('vps-init-pass').value, old_ssh_port: document.getElementById('vps-old-port').value.trim(), new_ssh_port: document.getElementById('vps-new-port').value.trim(), new_user: document.getElementById('vps-new-user').value.trim(), extra_ports: document.getElementById('vps-extra-ports').value.trim(), disable_root: document.getElementById('vps-no-root').checked, disable_pass: document.getElementById('vps-no-pass').checked, enable_ufw: document.getElementById('vps-ufw').checked }; }
document.getElementById('btn-vps-preview').addEventListener('click', async () => { const b = document.getElementById('vps-output'); b.style.display = 'block'; b.textContent = await goApp().PreviewVPSSecurityScript(collectVPS()); });
document.getElementById('btn-vps-run').addEventListener('click', async () => { const b = document.getElementById('vps-output'); b.style.display = 'block'; b.textContent = "正在加固..."; try { b.textContent = await goApp().ExecuteVPSSecurityHardening(collectVPS()); loadLocalSSHHosts(); } catch (e) { b.textContent = "❌ " + e; } });
document.getElementById('btn-vps-status').addEventListener('click', async () => { const b = document.getElementById('vps-output'); b.style.display = 'block'; try { b.textContent = await goApp().RunVPSRemoteCommand(getVPSTarget(), `uname -a && uptime && free -h && (sudo ufw status 2>/dev/null || true)`); } catch (e) { b.textContent = "❌ " + e; } });

function collectSB() { return { inbound_mode: document.getElementById('sb-in-mode').value, mixed_port: parseInt(document.getElementById('sb-in-port').value || "7890"), allow_lan: document.getElementById('sb-allow-lan').checked, protocol: document.getElementById('sb-proto').value, server_addr: document.getElementById('sb-server').value.trim(), server_port: parseInt(document.getElementById('sb-port').value || "443"), uuid_or_pass: document.getElementById('sb-uuid').value.trim(), flow: "xtls-rprx-vision", sni: document.getElementById('sb-sni').value.trim(), reality_pubkey: document.getElementById('sb-pbk').value.trim(), reality_sid: document.getElementById('sb-sid').value.trim(), hy2_up_mbps: 100, hy2_down_mbps: 200, bypass_cn: document.getElementById('sb-bypass-cn').checked, enable_ai_filter: document.getElementById('sb-ai-filter').checked, use_warp_for_ai: document.getElementById('sb-warp-ai').checked }; }
document.getElementById('btn-sb-parse').addEventListener('click', async () => { try { const c = await goApp().ParseSingboxURI(document.getElementById('sb-uri-input').value); document.getElementById('sb-proto').value = c.protocol; document.getElementById('sb-server').value = c.server_addr; document.getElementById('sb-port').value = c.server_port; document.getElementById('sb-uuid').value = c.uuid_or_pass; document.getElementById('sb-sni').value = c.sni || ""; document.getElementById('sb-pbk').value = c.reality_pubkey || ""; document.getElementById('sb-sid').value = c.reality_sid || ""; alert("✨ 解析成功！"); } catch (e) { alert(e); } });
document.getElementById('btn-sb-preview').addEventListener('click', async () => { const b = document.getElementById('sb-preview-box'); b.style.display = 'block'; b.textContent = await goApp().PreviewSingboxConfig(collectSB()); });
document.getElementById('btn-sb-export').addEventListener('click', async () => { const p = await goApp().ExportSingboxConfig(collectSB()); if (p) alert("已导出: " + p); });
document.getElementById('btn-sb-deploy').addEventListener('click', async () => { const b = document.getElementById('sb-preview-box'); b.style.display = 'block'; b.textContent = "正在推送部署..."; try { b.textContent = await goApp().DeploySingboxRemote(getVPSTarget(), collectSB()); } catch (e) { b.textContent = "❌ " + e; } });

function collectOW() { return { branch: document.getElementById('ow-branch').value, target_arch: document.getElementById('ow-arch').value, feed_source: document.getElementById('ow-feed').value, third_party: Array.from(document.querySelectorAll('.ow-tp:checked')).map(el => el.value), lan_ip: document.getElementById('ow-ip').value.trim(), hostname: document.getElementById('ow-hostname').value.trim(), default_theme: document.getElementById('ow-theme').value, rootfs_size_mb: 512, selected_pkgs: ["luci-app-ddns", "luci-app-upnp"] }; }
document.getElementById('btn-ow-preview').addEventListener('click', async () => { const b = await goApp().PreviewOpenWrtScripts(collectOW()); const el = document.getElementById('ow-script-preview'); el.style.display = 'block'; el.textContent = b.diy_part1 + "\n" + b.diy_part2 + "\n" + b.dot_config; });
document.getElementById('btn-ow-export').addEventListener('click', async () => { const d = await goApp().ExportOpenWrtBundle(collectOW()); if (d) alert("已导出至: " + d); });
document.getElementById('btn-ow-docker').addEventListener('click', async () => { try { await goApp().StartOpenWrtDockerBuild(collectOW()); } catch (e) { alert(e); } });
document.getElementById('btn-ow-gh-push').addEventListener('click', async () => { try { const u = await goApp().PushOpenWrtToGitHub({ token: document.getElementById('gh-token').value, repo: document.getElementById('gh-repo').value, branch: "main", fw_config: collectOW() }); alert("云端编译已启动: " + u); } catch (e) { alert(e); } });
document.getElementById('btn-ow-ssh').addEventListener('click', async () => { const b = document.getElementById('ow-ssh-output'); b.style.display = 'block'; try { b.textContent = await goApp().ProvisionOpenWrtSSH({ host: document.getElementById('ssh-host').value, port: "22", user: "root", password: document.getElementById('ssh-pass').value, feed_source: document.getElementById('ow-feed').value, bypass_mode: document.getElementById('ssh-bypass').checked, main_gateway: document.getElementById('ssh-gw').value, install_pkgs: document.getElementById('ssh-pkgs').value }); } catch (e) { b.textContent = "❌ " + e; } });

function collectPVE() {
  return {
    pve_ssh: { host: document.getElementById('pve-host').value.trim(), port: "22", user: "root", password: document.getElementById('pve-pass').value },
    ikuai_vmid: document.getElementById('pve-ikuai-id').value.trim(),
    openwrt_vmid: document.getElementById('pve-ow-id').value.trim(),
    openwrt_ssh: { host: document.getElementById('pve-ow-ip').value.trim(), port: "22", user: "root", password: document.getElementById('pve-ow-pass').value },
    zorin_vmid: document.getElementById('pve-zorin-id').value.trim(),
    zorin_ssh: { host: document.getElementById('pve-zorin-ip').value.trim(), port: "22", user: document.getElementById('pve-zorin-user').value.trim() || "ubuntu", password: document.getElementById('pve-zorin-pass').value },
    repo_source: document.getElementById('pve-repo-source').value
  };
}
document.getElementById('btn-pve-order').addEventListener('click', async () => { const b = document.getElementById('pve-output'); b.textContent = "正在锁定启动顺序..."; try { b.textContent = await goApp().PVEConfigureBootOrder(collectPVE()); } catch (e) { b.textContent = "❌ " + e; } });
document.getElementById('btn-pve-host-up').addEventListener('click', async () => { const b = document.getElementById('pve-output'); b.textContent = "正在升级 PVE 宿主机..."; try { b.textContent = await goApp().PVEUpgradeHost(collectPVE()); } catch (e) { b.textContent = "❌ " + e; } });
document.getElementById('btn-pve-vm-up').addEventListener('click', async () => { const b = document.getElementById('pve-output'); b.textContent = "正在快照并升级虚拟机..."; try { b.textContent = await goApp().PVESafeUpgradeVMs(collectPVE()); } catch (e) { b.textContent = "❌ " + e; } });
document.getElementById('btn-pve-ow-fw').addEventListener('click', async () => { const b = document.getElementById('pve-output'); b.textContent = "请选择 OpenWrt 固件文件（自动打快照 -> 上传 -> sysupgrade -> 探针校验/回滚）..."; try { b.textContent = await goApp().PVEUpgradeOpenWrtFirmwareDialog(collectPVE()); } catch (e) { b.textContent = "❌ " + e; } });
document.getElementById('btn-pve-subboost').addEventListener('click', async () => { const b = document.getElementById('pve-output'); b.textContent = "正在 Zorin OS 上部署 SubBoost..."; try { b.textContent = await goApp().PVEDeploySubBoost(collectPVE().zorin_ssh); } catch (e) { b.textContent = "❌ " + e; } });
document.getElementById('btn-check-integrity').addEventListener('click', async () => { const box = document.getElementById('integrity-console'); box.style.display = 'block'; box.textContent = '正在核验...'; try { box.textContent = await goApp().CheckCodeIntegrity(); } catch (e) { box.textContent = '[FAIL] ' + String(e); } });

document.getElementById('btn-refresh-manifest').addEventListener('click', async () => { const box = document.getElementById('integrity-console'); box.style.display = 'block'; box.textContent = '正在刷新基准清单...'; try { box.textContent = await goApp().RefreshCodeManifest(); } catch (e) { box.textContent = '[FAIL] ' + String(e); } });

document.getElementById('btn-check-integrity-step4').addEventListener('click', async () => { const logBox = document.getElementById('log-console'); logBox.textContent = '正在核验...'; try { logBox.textContent = await goApp().CheckCodeIntegrity(); } catch (e) { logBox.textContent = '[FAIL] ' + String(e); } });
