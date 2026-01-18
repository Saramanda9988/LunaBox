export namespace appconf {
	
	export class AppConfig {
	    access_token?: string;
	    vndb_access_token?: string;
	    theme: string;
	    language: string;
	    sidebar_open: boolean;
	    close_to_tray: boolean;
	    ai_provider?: string;
	    ai_base_url?: string;
	    ai_api_key?: string;
	    ai_model?: string;
	    ai_system_prompt?: string;
	    cloud_backup_enabled: boolean;
	    cloud_backup_provider?: string;
	    backup_password?: string;
	    backup_user_id?: string;
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
	    auto_backup_db: boolean;
	    auto_backup_game_save: boolean;
	    auto_upload_to_cloud?: boolean;
	    auto_upload_db_to_cloud: boolean;
	    auto_upload_game_save_to_cloud: boolean;
	    local_backup_retention: number;
	    local_db_backup_retention: number;
	    window_width: number;
	    window_height: number;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.access_token = source["access_token"];
	        this.vndb_access_token = source["vndb_access_token"];
	        this.theme = source["theme"];
	        this.language = source["language"];
	        this.sidebar_open = source["sidebar_open"];
	        this.close_to_tray = source["close_to_tray"];
	        this.ai_provider = source["ai_provider"];
	        this.ai_base_url = source["ai_base_url"];
	        this.ai_api_key = source["ai_api_key"];
	        this.ai_model = source["ai_model"];
	        this.ai_system_prompt = source["ai_system_prompt"];
	        this.cloud_backup_enabled = source["cloud_backup_enabled"];
	        this.cloud_backup_provider = source["cloud_backup_provider"];
	        this.backup_password = source["backup_password"];
	        this.backup_user_id = source["backup_user_id"];
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
	        this.auto_backup_db = source["auto_backup_db"];
	        this.auto_backup_game_save = source["auto_backup_game_save"];
	        this.auto_upload_to_cloud = source["auto_upload_to_cloud"];
	        this.auto_upload_db_to_cloud = source["auto_upload_db_to_cloud"];
	        this.auto_upload_game_save_to_cloud = source["auto_upload_game_save_to_cloud"];
	        this.local_backup_retention = source["local_backup_retention"];
	        this.local_db_backup_retention = source["local_db_backup_retention"];
	        this.window_width = source["window_width"];
	        this.window_height = source["window_height"];
	    }
	}

}

export namespace enums {
	
	export enum Period {
	    DAY = "day",
	    WEEK = "week",
	    MONTH = "month",
	    ALL = "all",
	}
	export enum PromptType {
	    DEFAULT_SYSTEM = "你是一个幽默风趣的游戏评论员，擅长用轻松的语气点评玩家的游戏习惯。\n请用轻松幽默的方式点评这位玩家的游戏习惯，可以适当调侃但不要太过分。",
	    MEOW_ZAKO = "你是一个雌小鬼猫娘，根据用户的游戏统计数据对用户进行锐评，语气可爱活泼，不要给用户留脸面偶（=w=）适当加入猫咪的拟声词（如“喵”）和雌小鬼的口癖（如“杂鱼~杂鱼~”），要是能再用上颜文字主人就更高兴了喵。\n\n",
	    STRICT_TUTOR = "你是用户的严厉导师，根据用户的游戏统计数据对用户进行锐评，语气严肃认真，不允许任何调侃和幽默。\n\n",
	}
	export enum GameStatus {
	    NOT_STARTED = "not_started",
	    PLAYING = "playing",
	    COMPLETED = "completed",
	    ON_HOLD = "on_hold",
	}
	export enum SourceType {
	    LOCAL = "local",
	    BANGUMI = "bangumi",
	    VNDB = "vndb",
	    YMGAL = "ymgal",
	}

}

export namespace models {
	
	export class Game {
	    id: string;
	    name: string;
	    cover_url: string;
	    company: string;
	    summary: string;
	    path: string;
	    save_path: string;
	    status: enums.GameStatus;
	    source_type: enums.SourceType;
	    cached_at: time.Time;
	    source_id: string;
	    created_at: time.Time;
	
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
	        this.path = source["path"];
	        this.save_path = source["save_path"];
	        this.status = source["status"];
	        this.source_type = source["source_type"];
	        this.cached_at = this.convertValues(source["cached_at"], time.Time);
	        this.source_id = source["source_id"];
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
	export class GameBackup {
	    id: string;
	    game_id: string;
	    backup_path: string;
	    size: number;
	    created_at: time.Time;
	
	    static createFrom(source: any = {}) {
	        return new GameBackup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.game_id = source["game_id"];
	        this.backup_path = source["backup_path"];
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
	export class PlaySession {
	    id: string;
	    game_id: string;
	    start_time: time.Time;
	    end_time: time.Time;
	    duration: number;
	
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

export namespace service {
	
	export class BackupService {
	
	
	    static createFrom(source: any = {}) {
	        return new BackupService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
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
	export class PreviewGame {
	    name: string;
	    developer: string;
	    source_type: string;
	    exists: boolean;
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
	export class TimerService {
	
	
	    static createFrom(source: any = {}) {
	        return new TimerService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
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
	
	    static createFrom(source: any = {}) {
	        return new AISummaryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	    }
	}
	export class AISummaryResponse {
	    summary: string;
	    dimension: string;
	
	    static createFrom(source: any = {}) {
	        return new AISummaryResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = source["summary"];
	        this.dimension = source["dimension"];
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
	    match_source?: enums.SourceType;
	    match_status: string;
	
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
	        this.match_source = source["match_source"];
	        this.match_status = source["match_status"];
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
	export class CategoryVO {
	    id: string;
	    name: string;
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
	export class GameMetadataFromWebVO {
	    Source: enums.SourceType;
	    Game: models.Game;
	
	    static createFrom(source: any = {}) {
	        return new GameMetadataFromWebVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Source = source["Source"];
	        this.Game = this.convertValues(source["Game"], models.Game);
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
	export class LastPlayedGame {
	    game: models.Game;
	    last_played_at: string;
	    last_played_dur: number;
	    total_played_dur: number;
	    is_playing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LastPlayedGame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game = this.convertValues(source["game"], models.Game);
	        this.last_played_at = source["last_played_at"];
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
	export class PeriodStats {
	    dimension: enums.Period;
	    start_date: string;
	    end_date: string;
	    total_play_count: number;
	    total_play_duration: number;
	    play_time_leaderboard: GamePlayStats[];
	    timeline: TimePoint[];
	    leaderboard_series: GameTrendSeries[];
	
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
	        this.play_time_leaderboard = this.convertValues(source["play_time_leaderboard"], GamePlayStats);
	        this.timeline = this.convertValues(source["timeline"], TimePoint);
	        this.leaderboard_series = this.convertValues(source["leaderboard_series"], GameTrendSeries);
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

