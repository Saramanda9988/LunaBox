export namespace appconf {
	
	export class AppConfig {
	    access_token?: string;
	    bangumi_refresh_token?: string;
	    bangumi_token_expires_at?: string;
	    bangumi_authorized_user_id?: string;
	    bangumi_authorized_username?: string;
	    bangumi_authorized_avatar_url?: string;
	    bangumi_auth_error?: string;
	    bangumi_status_push_enabled?: boolean;
	    vndb_access_token?: string;
	    metadata_sources?: string[];
	    allow_duplicate_metadata_import: boolean;
	    theme: string;
	    language: string;
	    sidebar_open: boolean;
	    close_to_tray: boolean;
	    ai_provider?: string;
	    ai_base_url?: string;
	    ai_api_key?: string;
	    ai_model?: string;
	    ai_system_prompt?: string;
	    ai_spoiler_level?: string;
	    ai_web_search: boolean;
	    ai_context_window?: number;
	    tavily_api_key?: string;
	    mcp_enabled: boolean;
	    mcp_port?: number;
	    cloud_backup_enabled: boolean;
	    cloud_backup_provider?: string;
	    backup_password?: string;
	    backup_user_id?: string;
	    cloud_sync_enabled: boolean;
	    auto_cloud_sync_enabled: boolean;
	    cloud_sync_interval_sec: number;
	    last_cloud_sync_time?: string;
	    last_cloud_sync_status?: string;
	    last_cloud_sync_error?: string;
	    s3_endpoint?: string;
	    s3_region?: string;
	    s3_bucket?: string;
	    s3_access_key?: string;
	    s3_secret_key?: string;
	    cloud_backup_retention?: number;
	    onedrive_client_id?: string;
	    onedrive_refresh_token?: string;
	    last_db_backup_time?: string;
	    pending_db_restore?: string;
	    last_full_backup_time?: string;
	    pending_full_restore?: string;
	    auto_backup_db: boolean;
	    auto_backup_game_save: boolean;
	    auto_upload_to_cloud?: boolean;
	    auto_upload_db_to_cloud: boolean;
	    auto_upload_game_save_to_cloud: boolean;
	    local_backup_retention: number;
	    local_db_backup_retention: number;
	    window_width: number;
	    window_height: number;
	    window_zoom_factor: number;
	    launch_at_login: boolean;
	    record_active_time_only: boolean;
	    check_update_on_startup: boolean;
	    update_check_url?: string;
	    last_update_check?: string;
	    skip_version?: string;
	    background_image?: string;
	    background_blur: number;
	    background_opacity: number;
	    background_enabled: boolean;
	    background_hide_game_cover: boolean;
	    background_hide_game_hero_cover: boolean;
	    background_is_light: boolean;
	    home_game_carousel_enabled: boolean;
	    home_game_carousel_interval_sec: number;
	    locale_emulator_path?: string;
	    magpie_path?: string;
	    auto_detect_game_process: boolean;
	    time_zone?: string;
	    game_library_path?: string;
	    batch_import_scan_preset?: string;
	    batch_import_hierarchy_depth: number;
	    batch_import_preferred_source?: string;
	    network_proxy_mode?: string;
	    network_proxy_url?: string;
	    show_nsfw_tags: boolean;
	    enable_tag_translation: boolean;
	    scraped_tag_limit: number;
	    show_sort_field_on_cover: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.access_token = source["access_token"];
	        this.bangumi_refresh_token = source["bangumi_refresh_token"];
	        this.bangumi_token_expires_at = source["bangumi_token_expires_at"];
	        this.bangumi_authorized_user_id = source["bangumi_authorized_user_id"];
	        this.bangumi_authorized_username = source["bangumi_authorized_username"];
	        this.bangumi_authorized_avatar_url = source["bangumi_authorized_avatar_url"];
	        this.bangumi_auth_error = source["bangumi_auth_error"];
	        this.bangumi_status_push_enabled = source["bangumi_status_push_enabled"];
	        this.vndb_access_token = source["vndb_access_token"];
	        this.metadata_sources = source["metadata_sources"];
	        this.allow_duplicate_metadata_import = source["allow_duplicate_metadata_import"];
	        this.theme = source["theme"];
	        this.language = source["language"];
	        this.sidebar_open = source["sidebar_open"];
	        this.close_to_tray = source["close_to_tray"];
	        this.ai_provider = source["ai_provider"];
	        this.ai_base_url = source["ai_base_url"];
	        this.ai_api_key = source["ai_api_key"];
	        this.ai_model = source["ai_model"];
	        this.ai_system_prompt = source["ai_system_prompt"];
	        this.ai_spoiler_level = source["ai_spoiler_level"];
	        this.ai_web_search = source["ai_web_search"];
	        this.ai_context_window = source["ai_context_window"];
	        this.tavily_api_key = source["tavily_api_key"];
	        this.mcp_enabled = source["mcp_enabled"];
	        this.mcp_port = source["mcp_port"];
	        this.cloud_backup_enabled = source["cloud_backup_enabled"];
	        this.cloud_backup_provider = source["cloud_backup_provider"];
	        this.backup_password = source["backup_password"];
	        this.backup_user_id = source["backup_user_id"];
	        this.cloud_sync_enabled = source["cloud_sync_enabled"];
	        this.auto_cloud_sync_enabled = source["auto_cloud_sync_enabled"];
	        this.cloud_sync_interval_sec = source["cloud_sync_interval_sec"];
	        this.last_cloud_sync_time = source["last_cloud_sync_time"];
	        this.last_cloud_sync_status = source["last_cloud_sync_status"];
	        this.last_cloud_sync_error = source["last_cloud_sync_error"];
	        this.s3_endpoint = source["s3_endpoint"];
	        this.s3_region = source["s3_region"];
	        this.s3_bucket = source["s3_bucket"];
	        this.s3_access_key = source["s3_access_key"];
	        this.s3_secret_key = source["s3_secret_key"];
	        this.cloud_backup_retention = source["cloud_backup_retention"];
	        this.onedrive_client_id = source["onedrive_client_id"];
	        this.onedrive_refresh_token = source["onedrive_refresh_token"];
	        this.last_db_backup_time = source["last_db_backup_time"];
	        this.pending_db_restore = source["pending_db_restore"];
	        this.last_full_backup_time = source["last_full_backup_time"];
	        this.pending_full_restore = source["pending_full_restore"];
	        this.auto_backup_db = source["auto_backup_db"];
	        this.auto_backup_game_save = source["auto_backup_game_save"];
	        this.auto_upload_to_cloud = source["auto_upload_to_cloud"];
	        this.auto_upload_db_to_cloud = source["auto_upload_db_to_cloud"];
	        this.auto_upload_game_save_to_cloud = source["auto_upload_game_save_to_cloud"];
	        this.local_backup_retention = source["local_backup_retention"];
	        this.local_db_backup_retention = source["local_db_backup_retention"];
	        this.window_width = source["window_width"];
	        this.window_height = source["window_height"];
	        this.window_zoom_factor = source["window_zoom_factor"];
	        this.launch_at_login = source["launch_at_login"];
	        this.record_active_time_only = source["record_active_time_only"];
	        this.check_update_on_startup = source["check_update_on_startup"];
	        this.update_check_url = source["update_check_url"];
	        this.last_update_check = source["last_update_check"];
	        this.skip_version = source["skip_version"];
	        this.background_image = source["background_image"];
	        this.background_blur = source["background_blur"];
	        this.background_opacity = source["background_opacity"];
	        this.background_enabled = source["background_enabled"];
	        this.background_hide_game_cover = source["background_hide_game_cover"];
	        this.background_hide_game_hero_cover = source["background_hide_game_hero_cover"];
	        this.background_is_light = source["background_is_light"];
	        this.home_game_carousel_enabled = source["home_game_carousel_enabled"];
	        this.home_game_carousel_interval_sec = source["home_game_carousel_interval_sec"];
	        this.locale_emulator_path = source["locale_emulator_path"];
	        this.magpie_path = source["magpie_path"];
	        this.auto_detect_game_process = source["auto_detect_game_process"];
	        this.time_zone = source["time_zone"];
	        this.game_library_path = source["game_library_path"];
	        this.batch_import_scan_preset = source["batch_import_scan_preset"];
	        this.batch_import_hierarchy_depth = source["batch_import_hierarchy_depth"];
	        this.batch_import_preferred_source = source["batch_import_preferred_source"];
	        this.network_proxy_mode = source["network_proxy_mode"];
	        this.network_proxy_url = source["network_proxy_url"];
	        this.show_nsfw_tags = source["show_nsfw_tags"];
	        this.enable_tag_translation = source["enable_tag_translation"];
	        this.scraped_tag_limit = source["scraped_tag_limit"];
	        this.show_sort_field_on_cover = source["show_sort_field_on_cover"];
	    }
	}

}

export namespace enums {
	
	export enum GameListSortBy {
	    NAME = "name",
	    LAST_PLAYED_AT = "last_played_at",
	    CREATED_AT = "created_at",
	    RATING = "rating",
	    RELEASE_DATE = "release_date",
	}
	export enum SortOrder {
	    ASC = "asc",
	    DESC = "desc",
	}
	export enum SourceType {
	    LOCAL = "local",
	    BANGUMI = "bangumi",
	    VNDB = "vndb",
	    YMGAL = "ymgal",
	    STEAM = "steam",
	    DLSITE = "dlsite",
	    EROGAMESCAPE = "erogamescape",
	}
	export enum Period {
	    DAY = "day",
	    WEEK = "week",
	    MONTH = "month",
	    YEAR = "year",
	    ALL = "all",
	}
	export enum PromptType {
	    DEFAULT_SYSTEM = "你是一个幽默风趣的游戏评论员，擅长用轻松的语气点评玩家的游戏习惯。\n请用轻松幽默的方式点评这位玩家的游戏习惯，可以适当调侃但不要太过分。",
	    MEOW_ZAKO = "你是一个雌小鬼猫娘，根据用户的游戏统计数据对用户进行锐评，语气可爱活泼，不要给用户留脸面偶（=w=）适当加入猫咪的拟声词（如“喵”）和雌小鬼的口癖（如“杂鱼~杂鱼~”），要是能再用上颜文字主人就更高兴了喵。\n\n",
	    STRICT_TUTOR = "你是用户的严厉导师，根据用户的游戏统计数据对用户进行锐评，语气严肃认真，不允许任何调侃和幽默。\n\n",
	}
	export enum GameStatus {
	    NOT_STARTED = "not_started",
	    WANT_TO_PLAY = "want_to_play",
	    PLAYING = "playing",
	    COMPLETED = "completed",
	    ON_HOLD = "on_hold",
	}

}

export namespace http {
	
	export class Client {
	    Transport: any;
	    Jar: any;
	    Timeout: number;
	
	    static createFrom(source: any = {}) {
	        return new Client(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Transport = source["Transport"];
	        this.Jar = source["Jar"];
	        this.Timeout = source["Timeout"];
	    }
	}

}

export namespace metadata {
	
	export class TagItem {
	    Name: string;
	    Source: string;
	    Weight: number;
	    IsSpoiler: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TagItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Source = source["Source"];
	        this.Weight = source["Weight"];
	        this.IsSpoiler = source["IsSpoiler"];
	    }
	}

}

export namespace models {
	
	export class Game {
	    id: string;
	    name: string;
	    cover_url: string;
	    company: string;
	    summary: string;
	    rating: number;
	    release_date: string;
	    path: string;
	    save_path: string;
	    process_name: string;
	    status: enums.GameStatus;
	    source_type: enums.SourceType;
	    cached_at: time.Time;
	    source_id: string;
	    created_at: time.Time;
	    updated_at: time.Time;
	    use_locale_emulator: boolean;
	    use_magpie: boolean;
	    metadata_locked: boolean;
	    last_played_at?: time.Time;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.cover_url = source["cover_url"];
	        this.company = source["company"];
	        this.summary = source["summary"];
	        this.rating = source["rating"];
	        this.release_date = source["release_date"];
	        this.path = source["path"];
	        this.save_path = source["save_path"];
	        this.process_name = source["process_name"];
	        this.status = source["status"];
	        this.source_type = source["source_type"];
	        this.cached_at = this.convertValues(source["cached_at"], time.Time);
	        this.source_id = source["source_id"];
	        this.created_at = this.convertValues(source["created_at"], time.Time);
	        this.updated_at = this.convertValues(source["updated_at"], time.Time);
	        this.use_locale_emulator = source["use_locale_emulator"];
	        this.use_magpie = source["use_magpie"];
	        this.metadata_locked = source["metadata_locked"];
	        this.last_played_at = this.convertValues(source["last_played_at"], time.Time);
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
	export class GameBackup {
	    path: string;
	    name: string;
	    game_id: string;
	    size: number;
	    created_at: time.Time;
	
	    static createFrom(source: any = {}) {
	        return new GameBackup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.game_id = source["game_id"];
	        this.size = source["size"];
	        this.created_at = this.convertValues(source["created_at"], time.Time);
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
	export class GameProgress {
	    id: string;
	    game_id: string;
	    chapter: string;
	    route: string;
	    progress_note: string;
	    spoiler_boundary: string;
	    updated_at: time.Time;
	
	    static createFrom(source: any = {}) {
	        return new GameProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.game_id = source["game_id"];
	        this.chapter = source["chapter"];
	        this.route = source["route"];
	        this.progress_note = source["progress_note"];
	        this.spoiler_boundary = source["spoiler_boundary"];
	        this.updated_at = this.convertValues(source["updated_at"], time.Time);
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
	export class GameTag {
	    id: string;
	    game_id: string;
	    name: string;
	    source: string;
	    weight: number;
	    is_spoiler: boolean;
	    created_at: time.Time;
	    updated_at: time.Time;
	
	    static createFrom(source: any = {}) {
	        return new GameTag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.game_id = source["game_id"];
	        this.name = source["name"];
	        this.source = source["source"];
	        this.weight = source["weight"];
	        this.is_spoiler = source["is_spoiler"];
	        this.created_at = this.convertValues(source["created_at"], time.Time);
	        this.updated_at = this.convertValues(source["updated_at"], time.Time);
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
	export class PlaySession {
	    id: string;
	    game_id: string;
	    start_time: time.Time;
	    end_time: time.Time;
	    duration: number;
	    updated_at: time.Time;
	
	    static createFrom(source: any = {}) {
	        return new PlaySession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.game_id = source["game_id"];
	        this.start_time = this.convertValues(source["start_time"], time.Time);
	        this.end_time = this.convertValues(source["end_time"], time.Time);
	        this.duration = source["duration"];
	        this.updated_at = this.convertValues(source["updated_at"], time.Time);
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
	export class User {
	    id: string;
	    created_at: time.Time;
	    default_backup_target: string;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], time.Time);
	        this.default_backup_target = source["default_backup_target"];
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

}

export namespace processutils {
	
	export class ProcessInfo {
	    name: string;
	    pid: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pid = source["pid"];
	    }
	}

}

export namespace service {
	
	export class BackupService {
	
	
	    static createFrom(source: any = {}) {
	        return new BackupService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class BangumiService {
	
	
	    static createFrom(source: any = {}) {
	        return new BangumiService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class ConfigService {
	
	
	    static createFrom(source: any = {}) {
	        return new ConfigService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class CoverImageDownloadItem {
	    GameID: string;
	    GameName: string;
	    CoverURL: string;
	
	    static createFrom(source: any = {}) {
	        return new CoverImageDownloadItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.GameID = source["GameID"];
	        this.GameName = source["GameName"];
	        this.CoverURL = source["CoverURL"];
	    }
	}
	export class DownloadTask {
	    id: string;
	    request: vo.InstallRequest;
	    status: string;
	    progress: number;
	    downloaded: number;
	    total: number;
	    error?: string;
	    file_path?: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.request = this.convertValues(source["request"], vo.InstallRequest);
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.downloaded = source["downloaded"];
	        this.total = source["total"];
	        this.error = source["error"];
	        this.file_path = source["file_path"];
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
	export class GameService {
	
	
	    static createFrom(source: any = {}) {
	        return new GameService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class ImportResult {
	    success: number;
	    skipped: number;
	    failed: number;
	    failed_names: string[];
	    skipped_names: string[];
	    sessions_imported: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.failed_names = source["failed_names"];
	        this.skipped_names = source["skipped_names"];
	        this.sessions_imported = source["sessions_imported"];
	    }
	}
	export class LaunchOptions {
	    UseLocaleEmulator?: boolean;
	    UseMagpie?: boolean;
	    RunAsAdmin?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LaunchOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.UseLocaleEmulator = source["UseLocaleEmulator"];
	        this.UseMagpie = source["UseMagpie"];
	        this.RunAsAdmin = source["RunAsAdmin"];
	    }
	}
	export class PortableCLIStatus {
	    available: boolean;
	    cliPath: string;
	    cliDir: string;
	    registered: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortableCLIStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.cliPath = source["cliPath"];
	        this.cliDir = source["cliDir"];
	        this.registered = source["registered"];
	    }
	}
	export class PortableProtocolStatus {
	    registered: boolean;
	    registeredPath: string;
	    currentPath: string;
	    upToDate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortableProtocolStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.registered = source["registered"];
	        this.registeredPath = source["registeredPath"];
	        this.currentPath = source["currentPath"];
	        this.upToDate = source["upToDate"];
	    }
	}
	export class PortableSetupStatus {
	    buildMode: string;
	    isPortable: boolean;
	    executablePath: string;
	    protocol: PortableProtocolStatus;
	    cli: PortableCLIStatus;
	
	    static createFrom(source: any = {}) {
	        return new PortableSetupStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.buildMode = source["buildMode"];
	        this.isPortable = source["isPortable"];
	        this.executablePath = source["executablePath"];
	        this.protocol = this.convertValues(source["protocol"], PortableProtocolStatus);
	        this.cli = this.convertValues(source["cli"], PortableCLIStatus);
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
	export class PreviewGame {
	    name: string;
	    developer: string;
	    source_type: string;
	    exists: boolean;
	    conflict_type: string;
	    existing_id: string;
	    existing_name: string;
	    add_time: time.Time;
	    has_path: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PreviewGame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.developer = source["developer"];
	        this.source_type = source["source_type"];
	        this.exists = source["exists"];
	        this.conflict_type = source["conflict_type"];
	        this.existing_id = source["existing_id"];
	        this.existing_name = source["existing_name"];
	        this.add_time = this.convertValues(source["add_time"], time.Time);
	        this.has_path = source["has_path"];
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
	export class SessionService {
	
	
	    static createFrom(source: any = {}) {
	        return new SessionService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class TagService {
	
	
	    static createFrom(source: any = {}) {
	        return new TagService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class UpdateCheckResult {
	    has_update: boolean;
	    current_ver: string;
	    latest_ver: string;
	    release_date: string;
	    changelog: string[];
	    downloads: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.has_update = source["has_update"];
	        this.current_ver = source["current_ver"];
	        this.latest_ver = source["latest_ver"];
	        this.release_date = source["release_date"];
	        this.changelog = source["changelog"];
	        this.downloads = source["downloads"];
	    }
	}

}

export namespace sql {
	
	export class DB {
	
	
	    static createFrom(source: any = {}) {
	        return new DB(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace time {
	
	export class Time {
	
	
	    static createFrom(source: any = {}) {
	        return new Time(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace vo {
	
	export class AISummaryRequest {
	    dimension: string;
	    spoiler_level?: string;
	
	    static createFrom(source: any = {}) {
	        return new AISummaryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	        this.spoiler_level = source["spoiler_level"];
	    }
	}
	export class AISummaryResponse {
	    summary: string;
	    dimension: string;
	    web_search_used: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AISummaryResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = source["summary"];
	        this.dimension = source["dimension"];
	        this.web_search_used = source["web_search_used"];
	    }
	}
	export class BangumiAuthStatus {
	    authorized: boolean;
	    needs_reauthorization: boolean;
	    legacy_token: boolean;
	    user_id: string;
	    username: string;
	    avatar_url: string;
	    access_token_expires_at: string;
	    last_error: string;
	
	    static createFrom(source: any = {}) {
	        return new BangumiAuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.authorized = source["authorized"];
	        this.needs_reauthorization = source["needs_reauthorization"];
	        this.legacy_token = source["legacy_token"];
	        this.user_id = source["user_id"];
	        this.username = source["username"];
	        this.avatar_url = source["avatar_url"];
	        this.access_token_expires_at = source["access_token_expires_at"];
	        this.last_error = source["last_error"];
	    }
	}
	export class BangumiProfile {
	    user_id: string;
	    username: string;
	    nickname: string;
	    avatar_url: string;
	    avatar_large: string;
	    avatar_medium: string;
	    avatar_small: string;
	
	    static createFrom(source: any = {}) {
	        return new BangumiProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user_id = source["user_id"];
	        this.username = source["username"];
	        this.nickname = source["nickname"];
	        this.avatar_url = source["avatar_url"];
	        this.avatar_large = source["avatar_large"];
	        this.avatar_medium = source["avatar_medium"];
	        this.avatar_small = source["avatar_small"];
	    }
	}
	export class BatchImportCandidate {
	    folder_path: string;
	    folder_name: string;
	    executables: string[];
	    selected_exe: string;
	    search_name: string;
	    is_selected: boolean;
	    matched_game?: models.Game;
	    matched_tags?: metadata.TagItem[];
	    match_source?: enums.SourceType;
	    match_status: string;
	    import_status: string;
	    skip_reason?: string;
	    existing_id?: string;
	    existing_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchImportCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder_path = source["folder_path"];
	        this.folder_name = source["folder_name"];
	        this.executables = source["executables"];
	        this.selected_exe = source["selected_exe"];
	        this.search_name = source["search_name"];
	        this.is_selected = source["is_selected"];
	        this.matched_game = this.convertValues(source["matched_game"], models.Game);
	        this.matched_tags = this.convertValues(source["matched_tags"], metadata.TagItem);
	        this.match_source = source["match_source"];
	        this.match_status = source["match_status"];
	        this.import_status = source["import_status"];
	        this.skip_reason = source["skip_reason"];
	        this.existing_id = source["existing_id"];
	        this.existing_name = source["existing_name"];
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
	export class BatchImportMetadataSourceError {
	    source: enums.SourceType;
	    error: string;
	    rate_limited: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BatchImportMetadataSourceError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.error = source["error"];
	        this.rate_limited = source["rate_limited"];
	    }
	}
	export class GameMetadataFromWebVO {
	    Source: enums.SourceType;
	    Game: models.Game;
	    Tags: metadata.TagItem[];
	
	    static createFrom(source: any = {}) {
	        return new GameMetadataFromWebVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Source = source["Source"];
	        this.Game = this.convertValues(source["Game"], models.Game);
	        this.Tags = this.convertValues(source["Tags"], metadata.TagItem);
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
	export class BatchImportMetadataMatchResult {
	    search_name: string;
	    preferred_source: enums.SourceType;
	    preferred_matched: boolean;
	    preferred_no_result: boolean;
	    preferred_rate_limited: boolean;
	    preferred_error?: string;
	    matches: GameMetadataFromWebVO[];
	    source_errors?: BatchImportMetadataSourceError[];
	
	    static createFrom(source: any = {}) {
	        return new BatchImportMetadataMatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.search_name = source["search_name"];
	        this.preferred_source = source["preferred_source"];
	        this.preferred_matched = source["preferred_matched"];
	        this.preferred_no_result = source["preferred_no_result"];
	        this.preferred_rate_limited = source["preferred_rate_limited"];
	        this.preferred_error = source["preferred_error"];
	        this.matches = this.convertValues(source["matches"], GameMetadataFromWebVO);
	        this.source_errors = this.convertValues(source["source_errors"], BatchImportMetadataSourceError);
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
	
	export class BatchImportScanOptions {
	    scan_mode: string;
	    scan_name_mode: string;
	    name_depth: number;
	    hierarchy_depth: number;
	
	    static createFrom(source: any = {}) {
	        return new BatchImportScanOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scan_mode = source["scan_mode"];
	        this.scan_name_mode = source["scan_name_mode"];
	        this.name_depth = source["name_depth"];
	        this.hierarchy_depth = source["hierarchy_depth"];
	    }
	}
	export class BatchImportScanResult {
	    candidates: BatchImportCandidate[];
	    skipped_candidates: BatchImportCandidate[];
	    total_detected: number;
	    skipped: number;
	
	    static createFrom(source: any = {}) {
	        return new BatchImportScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidates = this.convertValues(source["candidates"], BatchImportCandidate);
	        this.skipped_candidates = this.convertValues(source["skipped_candidates"], BatchImportCandidate);
	        this.total_detected = source["total_detected"];
	        this.skipped = source["skipped"];
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
	export class CategoryGameCandidateRequest {
	    category_id: string;
	    limit: number;
	    offset: number;
	    search_query: string;
	
	    static createFrom(source: any = {}) {
	        return new CategoryGameCandidateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category_id = source["category_id"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.search_query = source["search_query"];
	    }
	}
	export class CategoryGameListRequest {
	    category_id: string;
	    limit: number;
	    offset: number;
	    search_query: string;
	    status?: enums.GameStatus;
	    tags: string[];
	    sort_by: enums.GameListSortBy;
	    sort_order: enums.SortOrder;
	
	    static createFrom(source: any = {}) {
	        return new CategoryGameListRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category_id = source["category_id"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.search_query = source["search_query"];
	        this.status = source["status"];
	        this.tags = source["tags"];
	        this.sort_by = source["sort_by"];
	        this.sort_order = source["sort_order"];
	    }
	}
	export class CategoryVO {
	    id: string;
	    name: string;
	    emoji: string;
	    is_system: boolean;
	    created_at: time.Time;
	    updated_at: time.Time;
	    game_count: number;
	
	    static createFrom(source: any = {}) {
	        return new CategoryVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.emoji = source["emoji"];
	        this.is_system = source["is_system"];
	        this.created_at = this.convertValues(source["created_at"], time.Time);
	        this.updated_at = this.convertValues(source["updated_at"], time.Time);
	        this.game_count = source["game_count"];
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
	export class CloudBackupItem {
	    key: string;
	    name: string;
	    size: number;
	    created_at: time.Time;
	
	    static createFrom(source: any = {}) {
	        return new CloudBackupItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.created_at = this.convertValues(source["created_at"], time.Time);
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
	export class CloudBackupStatus {
	    enabled: boolean;
	    configured: boolean;
	    user_id: string;
	    provider: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudBackupStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.configured = source["configured"];
	        this.user_id = source["user_id"];
	        this.provider = source["provider"];
	    }
	}
	export class CloudSyncStatus {
	    enabled: boolean;
	    configured: boolean;
	    syncing: boolean;
	    last_sync_time: string;
	    last_sync_status: string;
	    last_sync_error: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudSyncStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.configured = source["configured"];
	        this.syncing = source["syncing"];
	        this.last_sync_time = source["last_sync_time"];
	        this.last_sync_status = source["last_sync_status"];
	        this.last_sync_error = source["last_sync_error"];
	    }
	}
	export class DBBackupInfo {
	    path: string;
	    name: string;
	    size: number;
	    created_at: time.Time;
	
	    static createFrom(source: any = {}) {
	        return new DBBackupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.created_at = this.convertValues(source["created_at"], time.Time);
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
	export class DBBackupStatus {
	    last_backup_time: string;
	    backups: DBBackupInfo[];
	
	    static createFrom(source: any = {}) {
	        return new DBBackupStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.last_backup_time = source["last_backup_time"];
	        this.backups = this.convertValues(source["backups"], DBBackupInfo);
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
	export class DailyPlayTime {
	    date: string;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyPlayTime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.duration = source["duration"];
	    }
	}
	export class DownloadImportState {
	    task_id: string;
	    imported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DownloadImportState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task_id = source["task_id"];
	        this.imported = source["imported"];
	    }
	}
	export class DownloadImportStateRequest {
	    task_id: string;
	    file_path: string;
	    meta_source: string;
	    meta_id: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadImportStateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task_id = source["task_id"];
	        this.file_path = source["file_path"];
	        this.meta_source = source["meta_source"];
	        this.meta_id = source["meta_id"];
	    }
	}
	export class GameDetailStats {
	    dimension: string;
	    start_date: string;
	    end_date: string;
	    total_play_count: number;
	    total_play_time: number;
	    today_play_time: number;
	    recent_play_history: DailyPlayTime[];
	
	    static createFrom(source: any = {}) {
	        return new GameDetailStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	        this.start_date = source["start_date"];
	        this.end_date = source["end_date"];
	        this.total_play_count = source["total_play_count"];
	        this.total_play_time = source["total_play_time"];
	        this.today_play_time = source["today_play_time"];
	        this.recent_play_history = this.convertValues(source["recent_play_history"], DailyPlayTime);
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
	export class GameListRequest {
	    limit: number;
	    offset: number;
	    search_query: string;
	    status?: enums.GameStatus;
	    tags: string[];
	    sort_by: enums.GameListSortBy;
	    sort_order: enums.SortOrder;
	
	    static createFrom(source: any = {}) {
	        return new GameListRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.search_query = source["search_query"];
	        this.status = source["status"];
	        this.tags = source["tags"];
	        this.sort_by = source["sort_by"];
	        this.sort_order = source["sort_order"];
	    }
	}
	export class GameListResponse {
	    games: models.Game[];
	    limit: number;
	    offset: number;
	    total: number;
	    has_more: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GameListResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.games = this.convertValues(source["games"], models.Game);
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.total = source["total"];
	        this.has_more = source["has_more"];
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
	
	export class GamePlayStats {
	    game_id: string;
	    game_name: string;
	    cover_url: string;
	    total_duration: number;
	
	    static createFrom(source: any = {}) {
	        return new GamePlayStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.game_name = source["game_name"];
	        this.cover_url = source["cover_url"];
	        this.total_duration = source["total_duration"];
	    }
	}
	export class GameStatsRequest {
	    game_id: string;
	    dimension: enums.Period;
	    start_date: string;
	    end_date: string;
	
	    static createFrom(source: any = {}) {
	        return new GameStatsRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.dimension = source["dimension"];
	        this.start_date = source["start_date"];
	        this.end_date = source["end_date"];
	    }
	}
	export class TimePoint {
	    label: string;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new TimePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.duration = source["duration"];
	    }
	}
	export class GameTrendSeries {
	    game_id: string;
	    game_name: string;
	    points: TimePoint[];
	
	    static createFrom(source: any = {}) {
	        return new GameTrendSeries(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.game_name = source["game_name"];
	        this.points = this.convertValues(source["points"], TimePoint);
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
	export class HeatmapCell {
	    date: string;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new HeatmapCell(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.duration = source["duration"];
	    }
	}
	export class LastPlayedGame {
	    game: models.Game;
	    last_played_at: time.Time;
	    last_played_dur: number;
	    total_played_dur: number;
	    is_playing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LastPlayedGame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game = this.convertValues(source["game"], models.Game);
	        this.last_played_at = this.convertValues(source["last_played_at"], time.Time);
	        this.last_played_dur = source["last_played_dur"];
	        this.total_played_dur = source["total_played_dur"];
	        this.is_playing = source["is_playing"];
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
	export class HomePageData {
	    last_played?: LastPlayedGame;
	    today_play_time_sec: number;
	    weekly_play_time_sec: number;
	
	    static createFrom(source: any = {}) {
	        return new HomePageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.last_played = this.convertValues(source["last_played"], LastPlayedGame);
	        this.today_play_time_sec = source["today_play_time_sec"];
	        this.weekly_play_time_sec = source["weekly_play_time_sec"];
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
	export class HourPlayPoint {
	    hour: number;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new HourPlayPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hour = source["hour"];
	        this.duration = source["duration"];
	    }
	}
	export class ImportMetadataDuplicateRequest {
	    source: enums.SourceType;
	    source_id: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportMetadataDuplicateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.source_id = source["source_id"];
	    }
	}
	export class ImportMetadataDuplicateResult {
	    source: enums.SourceType;
	    source_id: string;
	    exists: boolean;
	    existing_id?: string;
	    existing_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportMetadataDuplicateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.source_id = source["source_id"];
	        this.exists = source["exists"];
	        this.existing_id = source["existing_id"];
	        this.existing_name = source["existing_name"];
	    }
	}
	export class InstallRequest {
	    url: string;
	    file_name: string;
	    archive_format: string;
	    startup_path: string;
	    title: string;
	    download_source: string;
	    meta_source: string;
	    meta_id: string;
	    size: number;
	    checksum_algo: string;
	    checksum: string;
	    expires_at: number;
	
	    static createFrom(source: any = {}) {
	        return new InstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.file_name = source["file_name"];
	        this.archive_format = source["archive_format"];
	        this.startup_path = source["startup_path"];
	        this.title = source["title"];
	        this.download_source = source["download_source"];
	        this.meta_source = source["meta_source"];
	        this.meta_id = source["meta_id"];
	        this.size = source["size"];
	        this.checksum_algo = source["checksum_algo"];
	        this.checksum = source["checksum"];
	        this.expires_at = source["expires_at"];
	    }
	}
	
	export class MetadataRefreshResult {
	    total_games: number;
	    updated_games: number;
	    skipped_games: number;
	    failed_games: number;
	    locked_games: number;
	    failed_game_ids: string[];
	    failed_game_names: string[];
	
	    static createFrom(source: any = {}) {
	        return new MetadataRefreshResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_games = source["total_games"];
	        this.updated_games = source["updated_games"];
	        this.skipped_games = source["skipped_games"];
	        this.failed_games = source["failed_games"];
	        this.locked_games = source["locked_games"];
	        this.failed_game_ids = source["failed_game_ids"];
	        this.failed_game_names = source["failed_game_names"];
	    }
	}
	export class MetadataRequest {
	    source: enums.SourceType;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.id = source["id"];
	    }
	}
	export class WeekdayPlayPoint {
	    weekday: number;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new WeekdayPlayPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.weekday = source["weekday"];
	        this.duration = source["duration"];
	    }
	}
	export class TagPlayStats {
	    name: string;
	    total_duration: number;
	    game_count: number;
	
	    static createFrom(source: any = {}) {
	        return new TagPlayStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.total_duration = source["total_duration"];
	        this.game_count = source["game_count"];
	    }
	}
	export class PeriodStats {
	    dimension: enums.Period;
	    start_date: string;
	    end_date: string;
	    total_play_count: number;
	    total_play_duration: number;
	    total_games_count: number;
	    completed_games_count: number;
	    library_games_count: number;
	    all_sessions_count: number;
	    all_sessions_duration: number;
	    all_completed_games_count: number;
	    active_days: number;
	    avg_daily_duration: number;
	    avg_session_duration: number;
	    max_streak: number;
	    current_streak: number;
	    new_games_count: number;
	    play_time_leaderboard: GamePlayStats[];
	    timeline: TimePoint[];
	    leaderboard_series: GameTrendSeries[];
	    tag_distribution: TagPlayStats[];
	    heatmap: HeatmapCell[];
	    hourly_distribution: HourPlayPoint[];
	    weekday_distribution: WeekdayPlayPoint[];
	
	    static createFrom(source: any = {}) {
	        return new PeriodStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	        this.start_date = source["start_date"];
	        this.end_date = source["end_date"];
	        this.total_play_count = source["total_play_count"];
	        this.total_play_duration = source["total_play_duration"];
	        this.total_games_count = source["total_games_count"];
	        this.completed_games_count = source["completed_games_count"];
	        this.library_games_count = source["library_games_count"];
	        this.all_sessions_count = source["all_sessions_count"];
	        this.all_sessions_duration = source["all_sessions_duration"];
	        this.all_completed_games_count = source["all_completed_games_count"];
	        this.active_days = source["active_days"];
	        this.avg_daily_duration = source["avg_daily_duration"];
	        this.avg_session_duration = source["avg_session_duration"];
	        this.max_streak = source["max_streak"];
	        this.current_streak = source["current_streak"];
	        this.new_games_count = source["new_games_count"];
	        this.play_time_leaderboard = this.convertValues(source["play_time_leaderboard"], GamePlayStats);
	        this.timeline = this.convertValues(source["timeline"], TimePoint);
	        this.leaderboard_series = this.convertValues(source["leaderboard_series"], GameTrendSeries);
	        this.tag_distribution = this.convertValues(source["tag_distribution"], TagPlayStats);
	        this.heatmap = this.convertValues(source["heatmap"], HeatmapCell);
	        this.hourly_distribution = this.convertValues(source["hourly_distribution"], HourPlayPoint);
	        this.weekday_distribution = this.convertValues(source["weekday_distribution"], WeekdayPlayPoint);
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
	export class PeriodStatsRequest {
	    dimension: enums.Period;
	    start_date: string;
	    end_date: string;
	
	    static createFrom(source: any = {}) {
	        return new PeriodStatsRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	        this.start_date = source["start_date"];
	        this.end_date = source["end_date"];
	    }
	}
	export class ProtocolLaunchRequest {
	    game_id: string;
	    raw_url?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProtocolLaunchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.raw_url = source["raw_url"];
	    }
	}
	export class StatsGameTrend {
	    game_id: string;
	    game_name: string;
	    points: StatsTimePoint[];
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new StatsGameTrend(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.game_name = source["game_name"];
	        this.points = this.convertValues(source["points"], StatsTimePoint);
	        this.color = source["color"];
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
	export class StatsTimePoint {
	    label: string;
	    duration: number;
	    duration_str: string;
	    hours: number;
	
	    static createFrom(source: any = {}) {
	        return new StatsTimePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.duration = source["duration"];
	        this.duration_str = source["duration_str"];
	        this.hours = source["hours"];
	    }
	}
	export class StatsGameItem {
	    rank: number;
	    game_id: string;
	    game_name: string;
	    cover_url: string;
	    cover_base64: string;
	    total_duration: number;
	    duration_str: string;
	
	    static createFrom(source: any = {}) {
	        return new StatsGameItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.game_id = source["game_id"];
	        this.game_name = source["game_name"];
	        this.cover_url = source["cover_url"];
	        this.cover_base64 = source["cover_base64"];
	        this.total_duration = source["total_duration"];
	        this.duration_str = source["duration_str"];
	    }
	}
	export class StatsExportData {
	    export_time: string;
	    start_date: string;
	    end_date: string;
	    period: string;
	    total_play_count: number;
	    total_play_duration: number;
	    total_play_time_str: string;
	    leaderboard: StatsGameItem[];
	    timeline: StatsTimePoint[];
	    leaderboard_trend: StatsGameTrend[];
	    chart_labels: string;
	    chart_data: string;
	    game_trend_data: string;
	    ai_summary: string;
	    app_name: string;
	    app_version: string;
	
	    static createFrom(source: any = {}) {
	        return new StatsExportData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.export_time = source["export_time"];
	        this.start_date = source["start_date"];
	        this.end_date = source["end_date"];
	        this.period = source["period"];
	        this.total_play_count = source["total_play_count"];
	        this.total_play_duration = source["total_play_duration"];
	        this.total_play_time_str = source["total_play_time_str"];
	        this.leaderboard = this.convertValues(source["leaderboard"], StatsGameItem);
	        this.timeline = this.convertValues(source["timeline"], StatsTimePoint);
	        this.leaderboard_trend = this.convertValues(source["leaderboard_trend"], StatsGameTrend);
	        this.chart_labels = source["chart_labels"];
	        this.chart_data = source["chart_data"];
	        this.game_trend_data = source["game_trend_data"];
	        this.ai_summary = source["ai_summary"];
	        this.app_name = source["app_name"];
	        this.app_version = source["app_version"];
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
	export class RenderTemplateRequest {
	    template_id: string;
	    data: StatsExportData;
	
	    static createFrom(source: any = {}) {
	        return new RenderTemplateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.template_id = source["template_id"];
	        this.data = this.convertValues(source["data"], StatsExportData);
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
	export class RenderTemplateResponse {
	    html: string;
	
	    static createFrom(source: any = {}) {
	        return new RenderTemplateResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.html = source["html"];
	    }
	}
	
	
	
	
	
	export class TemplateInfo {
	    id: string;
	    name: string;
	    description: string;
	    author: string;
	    version: string;
	    preview: string;
	    is_builtin: boolean;
	    file_path: string;
	
	    static createFrom(source: any = {}) {
	        return new TemplateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.author = source["author"];
	        this.version = source["version"];
	        this.preview = source["preview"];
	        this.is_builtin = source["is_builtin"];
	        this.file_path = source["file_path"];
	    }
	}
	

}

