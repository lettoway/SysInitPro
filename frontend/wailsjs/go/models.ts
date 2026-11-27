export namespace core {
	
	export class HardwareInfo {
	    os_platform: string;
	    os_version: string;
	    os_role: string;
	    activation_status: string;
	    cpu: string;
	    gpu: string;
	    motherboard: string;
	    network_card: string;
	    missing_drivers: string[];
	
	    static createFrom(source: any = {}) {
	        return new HardwareInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os_platform = source["os_platform"];
	        this.os_version = source["os_version"];
	        this.os_role = source["os_role"];
	        this.activation_status = source["activation_status"];
	        this.cpu = source["cpu"];
	        this.gpu = source["gpu"];
	        this.motherboard = source["motherboard"];
	        this.network_card = source["network_card"];
	        this.missing_drivers = source["missing_drivers"];
	    }
	}
	export class PatchItem {
	    id: string;
	    title: string;
	    category: string;
	
	    static createFrom(source: any = {}) {
	        return new PatchItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.category = source["category"];
	    }
	}
	export class PreflightResult {
	    is_admin: boolean;
	    free_disk_gb: number;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new PreflightResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_admin = source["is_admin"];
	        this.free_disk_gb = source["free_disk_gb"];
	        this.warnings = source["warnings"];
	    }
	}

}

export namespace models {
	
	export class FirmwareConfig {
	    target: string;
	    branch: string;
	    lanIP: string;
	    rootPassword: string;
	    theme: string;
	    feeds: string[];
	    plugins: string[];
	    gitMirror: string;
	
	    static createFrom(source: any = {}) {
	        return new FirmwareConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.branch = source["branch"];
	        this.lanIP = source["lanIP"];
	        this.rootPassword = source["rootPassword"];
	        this.theme = source["theme"];
	        this.feeds = source["feeds"];
	        this.plugins = source["plugins"];
	        this.gitMirror = source["gitMirror"];
	    }
	}
	export class AppConfig {
	    openwrt: FirmwareConfig;
	    pveHost: string;
	    pveUser: string;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.openwrt = this.convertValues(source["openwrt"], FirmwareConfig);
	        this.pveHost = source["pveHost"];
	        this.pveUser = source["pveUser"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CatalogOption {
	    id: string;
	    name: string;
	    desc?: string;
	
	    static createFrom(source: any = {}) {
	        return new CatalogOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.desc = source["desc"];
	    }
	}
	export class FirmwareBundle {
	    diyPart1: string;
	    diyPart2: string;
	    dotConfig: string;
	    workflowYml: string;
	
	    static createFrom(source: any = {}) {
	        return new FirmwareBundle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.diyPart1 = source["diyPart1"];
	        this.diyPart2 = source["diyPart2"];
	        this.dotConfig = source["dotConfig"];
	        this.workflowYml = source["workflowYml"];
	    }
	}
	
	export class PluginItem {
	    id: string;
	    name: string;
	    desc: string;
	    feed: string;
	    defaultOn: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PluginItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.desc = source["desc"];
	        this.feed = source["feed"];
	        this.defaultOn = source["defaultOn"];
	    }
	}
	export class OpenWrtCatalog {
	    targets: CatalogOption[];
	    branches: CatalogOption[];
	    themes: CatalogOption[];
	    feeds: CatalogOption[];
	    plugins: PluginItem[];
	
	    static createFrom(source: any = {}) {
	        return new OpenWrtCatalog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.targets = this.convertValues(source["targets"], CatalogOption);
	        this.branches = this.convertValues(source["branches"], CatalogOption);
	        this.themes = this.convertValues(source["themes"], CatalogOption);
	        this.feeds = this.convertValues(source["feeds"], CatalogOption);
	        this.plugins = this.convertValues(source["plugins"], PluginItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class TaskStatus {
	    id: string;
	    name: string;
	    state: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.state = source["state"];
	        this.message = source["message"];
	    }
	}

}

export namespace network {
	
	export class MirrorNode {
	    name: string;
	    base_url: string;
	    latency: number;
	
	    static createFrom(source: any = {}) {
	        return new MirrorNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.base_url = source["base_url"];
	        this.latency = source["latency"];
	    }
	}

}

export namespace openwrt {
	
	export class ConflictWarning {
	    level: string;
	    title: string;
	    message: string;
	    plugins: string[];
	
	    static createFrom(source: any = {}) {
	        return new ConflictWarning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.title = source["title"];
	        this.message = source["message"];
	        this.plugins = source["plugins"];
	    }
	}

}

export namespace pve {
	
	export class PVEClusterConfig {
	    pve_ssh: sshx.SSHTarget;
	    ikuai_vmid: string;
	    openwrt_vmid: string;
	    openwrt_ssh: sshx.SSHTarget;
	    zorin_vmid: string;
	    zorin_ssh: sshx.SSHTarget;
	    repo_source: string;
	
	    static createFrom(source: any = {}) {
	        return new PVEClusterConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pve_ssh = this.convertValues(source["pve_ssh"], sshx.SSHTarget);
	        this.ikuai_vmid = source["ikuai_vmid"];
	        this.openwrt_vmid = source["openwrt_vmid"];
	        this.openwrt_ssh = this.convertValues(source["openwrt_ssh"], sshx.SSHTarget);
	        this.zorin_vmid = source["zorin_vmid"];
	        this.zorin_ssh = this.convertValues(source["zorin_ssh"], sshx.SSHTarget);
	        this.repo_source = source["repo_source"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProbeResult {
	    ok: boolean;
	    latency: string;
	    summary: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.latency = source["latency"];
	        this.summary = source["summary"];
	        this.error = source["error"];
	    }
	}

}

export namespace sshx {
	
	export class SSHTarget {
	    host: string;
	    port: string;
	    user: string;
	    password: string;
	    identity_file: string;
	
	    static createFrom(source: any = {}) {
	        return new SSHTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.password = source["password"];
	        this.identity_file = source["identity_file"];
	    }
	}

}

export namespace translator {
	
	export class SingboxFormConfig {
	    inbound_mode: string;
	    mixed_port: number;
	    allow_lan: boolean;
	    protocol: string;
	    server_addr: string;
	    server_port: number;
	    uuid_or_pass: string;
	    flow: string;
	    sni: string;
	    reality_pubkey: string;
	    reality_sid: string;
	    hy2_up_mbps: number;
	    hy2_down_mbps: number;
	    bypass_cn: boolean;
	    enable_ai_filter: boolean;
	    use_warp_for_ai: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SingboxFormConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inbound_mode = source["inbound_mode"];
	        this.mixed_port = source["mixed_port"];
	        this.allow_lan = source["allow_lan"];
	        this.protocol = source["protocol"];
	        this.server_addr = source["server_addr"];
	        this.server_port = source["server_port"];
	        this.uuid_or_pass = source["uuid_or_pass"];
	        this.flow = source["flow"];
	        this.sni = source["sni"];
	        this.reality_pubkey = source["reality_pubkey"];
	        this.reality_sid = source["reality_sid"];
	        this.hy2_up_mbps = source["hy2_up_mbps"];
	        this.hy2_down_mbps = source["hy2_down_mbps"];
	        this.bypass_cn = source["bypass_cn"];
	        this.enable_ai_filter = source["enable_ai_filter"];
	        this.use_warp_for_ai = source["use_warp_for_ai"];
	    }
	}
	export class VPSSecurityConfig {
	    server_alias: string;
	    server_ip: string;
	    init_user: string;
	    init_password: string;
	    old_ssh_port: string;
	    new_ssh_port: string;
	    new_user: string;
	    public_key: string;
	    disable_root: boolean;
	    disable_pass: boolean;
	    enable_ufw: boolean;
	    extra_ports: string;
	
	    static createFrom(source: any = {}) {
	        return new VPSSecurityConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_alias = source["server_alias"];
	        this.server_ip = source["server_ip"];
	        this.init_user = source["init_user"];
	        this.init_password = source["init_password"];
	        this.old_ssh_port = source["old_ssh_port"];
	        this.new_ssh_port = source["new_ssh_port"];
	        this.new_user = source["new_user"];
	        this.public_key = source["public_key"];
	        this.disable_root = source["disable_root"];
	        this.disable_pass = source["disable_pass"];
	        this.enable_ufw = source["enable_ufw"];
	        this.extra_ports = source["extra_ports"];
	    }
	}

}

